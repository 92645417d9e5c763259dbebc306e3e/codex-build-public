package runtime

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"codex-build/internal/core"
)

type RustSuiteMaterializationEvidence struct {
	Action            string
	InvalidationCause string
	Current           cargoIdentity
	EffectiveReuse    string
}

type cargoIdentity struct {
	Generation                                           uint64
	CargoVersion, ToolchainRoot, Target, SourceSelection string
}

type cargoPreparation struct {
	Materialization RustSuiteMaterializationEvidence
	SourceCommit    string `json:",omitempty"`
	stage           string
}

func validCargoMaterializationAction(value RustSuiteMaterializationEvidence) bool {
	switch value.Action + "/" + value.InvalidationCause {
	case "Initialized/IdentityAbsent":
		return value.Current.Generation == 1
	case "ReusedWorkspace/None":
		return value.Current.Generation > 0
	case "InvalidatedWorkspace/SourceChanged":
		return value.Current.Generation > 1
	}
	return false
}

func cargoWorktreeSelection(root, source string) (string, error) {
	selection, err := filepath.Rel(root, source)
	if err != nil || !filepath.IsLocal(selection) || selection == "." {
		return "", errors.Join(err, errors.New("source selection"))
	}
	return filepath.ToSlash(selection), nil
}

func prepareCargo(cache CacheProjection, current cargoIdentity, stage string, window deadline) (cargoPreparation, error) {
	if stage != "" && !listed("FinalProductRustSuite/baseline FinalProductRustSuite/full", stage) {
		return cargoPreparation{}, errors.New("Cargo stage identity differs")
	}
	for _, path := range []string{cache.CargoHome, cache.CargoBuild, cache.CargoTarget, cache.SccacheDirectory} {
		if err := noFollowPath("Cargo cache path", path); err != nil {
			return cargoPreparation{}, err
		}
	}
	previous, present, err := readCargoState(cache.CargoBuild)
	if err != nil {
		return cargoPreparation{}, err
	}
	action, cause := "Initialized", "IdentityAbsent"
	if present {
		requestedSource := current.SourceSelection
		current.Generation = previous.Generation
		current.SourceSelection = previous.SourceSelection
		if current != previous {
			return cargoPreparation{}, errors.New("selected Cargo cache state is incompatible")
		}
		current.SourceSelection = requestedSource
		action, cause = "ReusedWorkspace", "None"
		if requestedSource != previous.SourceSelection {
			if current.Generation == ^uint64(0) {
				return cargoPreparation{}, errors.New("Cargo source generation is exhausted")
			}
			current.Generation++
			action, cause = "InvalidatedWorkspace", "SourceChanged"
		}
	} else {
		before, err := optionalCargoBytes(window, cache.CargoTarget, cache.CargoBuild)
		if err != nil {
			return cargoPreparation{}, err
		}
		if before != 0 {
			return cargoPreparation{}, errors.New("selected Cargo cache state is absent beside existing bytes")
		}
		current.Generation, action, cause = 1, "Initialized", "IdentityAbsent"
	}
	preparation := cargoPreparation{Materialization: RustSuiteMaterializationEvidence{
		Action: action, InvalidationCause: cause, Current: current,
		EffectiveReuse: "None",
	}, stage: stage}
	if action == "ReusedWorkspace" {
		preparation.Materialization.EffectiveReuse = "CargoManaged"
	}
	return preparation, nil
}

func readCargoState(build string) (cargoIdentity, bool, error) {
	path := filepath.Join(build, "cargo-state.json")
	var fields map[string]json.RawMessage
	data, err := readCanonicalValue(path, &fields)
	if os.IsNotExist(err) {
		return cargoIdentity{}, false, nil
	}
	if err != nil {
		return cargoIdentity{}, false, err
	}
	var state cargoIdentity
	if len(fields) != 5 || fields["Generation"] == nil || fields["CargoVersion"] == nil || fields["ToolchainRoot"] == nil || fields["Target"] == nil || fields["SourceSelection"] == nil || json.Unmarshal(data, &state) != nil || !rustSuiteValidCargoIdentity(state) {
		return cargoIdentity{}, false, errors.New("selected Cargo cache state is not current")
	}
	return state, true, nil
}

func publishCargoState(build string, state cargoIdentity) error {
	data, err := core.CanonicalJSON(state)
	if err != nil {
		return err
	}
	return publishReplace(filepath.Join(build, "cargo-state.json"), data, 0o600)
}

func optionalCargoBytes(window deadline, paths ...string) (int64, error) {
	var total int64
	for _, path := range paths {
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return 0, errors.Join(err, errors.New("unsafe Cargo cache path: "+path))
		}
		bytes, bytesErr := regularFileBytes(window, path)
		if bytesErr != nil || bytes < 0 || bytes > int64(^uint64(0)>>1)-total {
			return 0, errors.Join(bytesErr, errors.New("Cargo cache byte measurement differs"))
		}
		total += bytes
	}
	return total, nil
}

func ensureCargoProjection(cache CacheProjection) error {
	for _, path := range []string{cache.CargoHome, cache.CargoBuild, cache.CargoTarget, cache.SccacheDirectory} {
		if err := ensureDirectoryNoFollow(path, 0o700); err != nil {
			return err
		}
	}
	return nil
}

func applySccacheReuse(preparation *cargoPreparation, data []byte) error {
	hit, err := sccacheReportedCacheHit(data)
	if err != nil {
		return err
	}
	switch {
	case preparation.Materialization.Action == "ReusedWorkspace":
		preparation.Materialization.EffectiveReuse = "CargoManaged"
	case preparation.Materialization.Action == "InvalidatedWorkspace" && hit:
		preparation.Materialization.EffectiveReuse = "RetainedCompatible"
	default:
		preparation.Materialization.EffectiveReuse = "None"
	}
	return nil
}

func validCargoReuse(value RustSuiteMaterializationEvidence) bool {
	if value.Action == "ReusedWorkspace" {
		return value.EffectiveReuse == "CargoManaged"
	}
	return value.EffectiveReuse == "None" || value.Action == "InvalidatedWorkspace" && value.EffectiveReuse == "RetainedCompatible"
}

func sccacheReportedCacheHit(data []byte) (bool, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var statistics map[string]any
	if err := decoder.Decode(&statistics); err != nil {
		return false, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return false, errors.Join(err, errors.New("trailing sccache statistics"))
	}
	statsValue, exists := statistics["stats"]
	if !exists {
		return false, nil
	}
	stats, valid := statsValue.(map[string]any)
	if !valid {
		return false, errors.New("invalid sccache stats object")
	}
	hitsValue, exists := stats["cache_hits"]
	if !exists {
		return false, nil
	}
	hits, valid := hitsValue.(map[string]any)
	if !valid {
		return false, errors.New("invalid sccache stats.cache_hits object")
	}
	return sccachePositiveCount(hits)
}

func sccachePositiveCount(value any) (bool, error) {
	switch value := value.(type) {
	case json.Number:
		count, err := value.Int64()
		if err != nil || count < 0 {
			return false, errors.Join(err, errors.New("invalid sccache cache-hit count"))
		}
		return count > 0, nil
	case map[string]any:
		anyHit := false
		for _, child := range value {
			hit, err := sccachePositiveCount(child)
			if err != nil {
				return false, err
			}
			anyHit = anyHit || hit
		}
		return anyHit, nil
	default:
		return false, errors.New("invalid sccache cache-hit statistics")
	}
}
