package runtime

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"codex-build/internal/core"
)

type outcomeTaskIndex struct {
	ActiveTask  []string
	OtherTasks  []string
	OverallTask []string
}

const (
	failureDomainNone    = "None"
	failureDomainInput   = "Input"
	failureDomainProduct = "Product"
	failureDomainHelper  = "Helper"
)

func validFailureDomain(value string) bool {
	return listed("None Input Product Helper", value)
}

func readCanonicalValue(path string, value any) ([]byte, error) {
	data, err := readJSONValue(path, value)
	if err != nil {
		return nil, err
	}
	canonical, canonicalErr := core.CanonicalJSON(value)
	if canonicalErr != nil || !bytes.Equal(data, canonical) {
		return nil, errors.Join(canonicalErr, errors.New("JSON is not canonical"))
	}
	return data, nil
}

func initializeAbsentJSON(path string, value any) error {
	current, _, exists, err := readOptionalRegular(path)
	if err != nil {
		return err
	}
	if exists {
		return consumePublicationSuffixes(path+".stage", path+".pending", current)
	}
	data, err := core.CanonicalJSON(value)
	if err == nil {
		err = publishNoReplace(path, data, 0o644)
	}
	return err
}

func readJSONValue(path string, value any) ([]byte, error) {
	data, err := readRegularNoFollow(path)
	if err == nil {
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		err = decoder.Decode(value)
		if err == nil {
			var trailing json.RawMessage
			if trailingErr := decoder.Decode(&trailing); !errors.Is(trailingErr, io.EOF) {
				err = errors.Join(trailingErr, errors.New("JSON has trailing content"))
			}
		}
	}
	return data, err
}

func validateJSONUniqueKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := validateJSONObjectKeys(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.Join(err, errors.New("JSON has trailing content"))
	}
	return nil
}

func validateJSONObjectKeys(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, object := token.(json.Delim)
	if !object {
		return nil
	}
	switch delimiter {
	case '{':
		fields := map[string]bool{}
		for decoder.More() {
			field, fieldErr := decoder.Token()
			name, named := field.(string)
			if fieldErr != nil || !named || fields[name] {
				return errors.Join(fieldErr, errors.New("JSON object fields differ"))
			}
			fields[name] = true
			if err := validateJSONObjectKeys(decoder); err != nil {
				return err
			}
		}
		closing, closingErr := decoder.Token()
		if closingErr != nil || closing != json.Delim('}') {
			return errors.Join(closingErr, errors.New("JSON object differs"))
		}
	case '[':
		for decoder.More() {
			if err := validateJSONObjectKeys(decoder); err != nil {
				return err
			}
		}
		closing, closingErr := decoder.Token()
		if closingErr != nil || closing != json.Delim(']') {
			return errors.Join(closingErr, errors.New("JSON array differs"))
		}
	default:
		return errors.New("JSON delimiter differs")
	}
	return nil
}

func activeTaskPath(root, tag, expected string) (string, error) {
	var index outcomeTaskIndex
	if _, err := readCanonicalValue(filepath.Join(root, "tasks", "index.json"), &index); err != nil {
		return "", err
	}
	if err := validateReleaseTaskIndex(root, index, ""); err != nil {
		return "", err
	}
	if len(index.ActiveTask) != 1 || len(index.OtherTasks) != 0 || index.ActiveTask[0] != releaseTaskName(tag) {
		return "", errors.New("ActiveTask must name the sole unfinished release tag")
	}
	path := filepath.Join(root, "tasks", index.ActiveTask[0])
	data, err := readRegularNoFollow(path)
	prefixes, prefixErr := taskSourcePrefixes(data, tag)
	if err != nil || prefixErr != nil || expected != "" && expected != prefixes[0] {
		return "", errors.Join(err, prefixErr, errors.New("active release source prefixes differ"))
	}
	return path, nil
}

func releaseTaskName(tag string) string {
	return "release-roll-" + tag + ".md"
}

func releaseTaskTag(name string) (string, error) {
	tag := strings.TrimSuffix(strings.TrimPrefix(name, "release-roll-"), ".md")
	if !core.ValidReleaseTag(tag) || strings.Contains(name, "-to-rust-v") || name != releaseTaskName(tag) {
		return "", errors.New("release task must use release-roll-<Tag>.md: " + name)
	}
	return tag, nil
}

func taskSourcePrefixes(data []byte, tag string) ([]string, error) {
	fields, err := taskGraphFields(data, "SourcePrefixes")
	if err != nil || fields["SourcePrefixes"] == "" {
		return nil, errors.Join(err, errors.New("release task requires explicit SourcePrefixes joined by comma-space"))
	}
	prefixes := strings.Split(fields["SourcePrefixes"], ", ")
	seen := map[string]bool{}
	for _, prefix := range prefixes {
		if !core.ValidReleaseTag(prefix) || seen[prefix] {
			return nil, errors.New("release task SourcePrefixes differ")
		}
		seen[prefix] = true
	}
	if prefixes[0] == tag {
		return nil, errors.New("primary predecessor must differ from the release tag")
	}
	return prefixes, nil
}

func validateReleaseTaskIndex(root string, index outcomeTaskIndex, completedTag string) error {
	if err := validateTaskIndex(root, index); err != nil {
		return err
	}
	if len(index.OtherTasks) != 0 {
		return errors.New("OtherTasks must be empty for the single current release")
	}
	indexed := map[string]bool{}
	for _, name := range index.ActiveTask {
		if _, err := releaseTaskTag(name); err != nil {
			return err
		}
		indexed[name] = true
	}
	entries, err := os.ReadDir(filepath.Join(root, "tasks"))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "release-roll-") && strings.HasSuffix(entry.Name(), ".md") && !indexed[entry.Name()] && (completedTag == "" || entry.Name() != releaseTaskName(completedTag)) {
			return errors.New("unindexed release task: " + entry.Name())
		}
	}
	return nil
}

func nextReleaseTaskIndex(index outcomeTaskIndex, completedTag string) (outcomeTaskIndex, error) {
	result := outcomeTaskIndex{ActiveTask: []string{}, OtherTasks: []string{}, OverallTask: append([]string{}, index.OverallTask...)}
	if len(index.ActiveTask) == 0 && len(index.OtherTasks) == 0 {
		return result, nil
	}
	if len(index.OtherTasks) != 0 || len(index.ActiveTask) != 1 || index.ActiveTask[0] != releaseTaskName(completedTag) {
		return result, errors.New("completed release must be the sole active task")
	}
	return result, nil
}

func validateTaskIndex(root string, index outcomeTaskIndex) error {
	if len(index.ActiveTask) > 1 || len(index.OverallTask) > 1 {
		return errors.New("TaskIndex differs")
	}
	seen := map[string]bool{}
	for _, group := range [][]string{index.ActiveTask, index.OtherTasks, index.OverallTask} {
		if group == nil {
			return errors.New("TaskIndex differs")
		}
		for position, name := range group {
			if position > 0 && group[position-1] >= name || seen[name] {
				return errors.New("TaskIndex differs")
			}
			seen[name] = true
			if filepath.Base(name) != name || !strings.HasSuffix(name, ".md") {
				return errors.New("TaskIndex differs")
			}
			info, err := os.Lstat(filepath.Join(root, "tasks", name))
			if err != nil || !info.Mode().IsRegular() {
				return errors.Join(err, errors.New("TaskIndex differs"))
			}
		}
	}
	return nil
}

func publishExactPostimage(path string, preimage, postimage []byte) error {
	current, mode, exists, err := readOptionalRegular(path)
	if err != nil {
		return errors.Join(err, errors.New("publication target differs"))
	}
	pending := path + ".pending"
	if exists && bytes.Equal(current, postimage) {
		if err := removePublicationFile(pending); err != nil {
			return err
		}
		return completeExactPostimage(path, postimage)
	}
	if !matchesTaskPreimage(current, exists, preimage) {
		return errors.New("preimage changed before publication")
	}
	pendingData, _, pendingExists, pendingErr := readOptionalRegular(pending)
	if pendingErr != nil {
		return errors.Join(pendingErr, errors.New("publication pending differs"))
	}
	if pendingExists && !bytes.Equal(pendingData, postimage) {
		if err := removePublicationFile(pending); err != nil {
			return err
		}
		if err := syncDirectory(filepath.Dir(path)); err != nil {
			return err
		}
		pendingExists = false
	}
	if !pendingExists {
		if !exists {
			mode = 0o644
		}
		if err := writeTaskPending(pending, postimage, mode); err != nil {
			return err
		}
	}
	current, _, exists, err = readOptionalRegular(path)
	if err != nil {
		return errors.Join(err, errors.New("publication target differs"))
	}
	if !matchesTaskPreimage(current, exists, preimage) {
		return errors.New("preimage changed before publication")
	}
	if err := os.Rename(pending, path); err != nil {
		return err
	}
	return completeExactPostimage(path, postimage)
}

func completeExactPostimage(path string, postimage []byte) error {
	if err := syncDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	return equalRecord(path, postimage)
}

func readOptionalRegular(path string) ([]byte, os.FileMode, bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, 0, false, nil
	}
	if err != nil || !info.Mode().IsRegular() {
		return nil, 0, false, errors.Join(err, fmt.Errorf("not a regular file: %s", path))
	}
	data, err := readRegularNoFollow(path)
	return data, info.Mode().Perm(), true, err
}

func matchesTaskPreimage(current []byte, exists bool, preimage []byte) bool {
	return exists == (preimage != nil) && (!exists || bytes.Equal(current, preimage))
}

func writeTaskPending(path string, postimage []byte, mode os.FileMode) (err error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, mode)
	if err != nil {
		return err
	}
	if err = file.Chmod(mode); err == nil {
		var written int
		written, err = file.Write(postimage)
		if err == nil && written != len(postimage) {
			err = io.ErrShortWrite
		}
	}
	err = errors.Join(err, syncClose(file))
	if err != nil {
		return errors.Join(err, removePublicationFile(path))
	}
	return syncDirectory(filepath.Dir(path))
}

func publishNoReplace(path string, data []byte, mode os.FileMode) error {
	stage, pending := path+".stage", path+".pending"
	if err := equalRecord(path, data); err == nil {
		return consumePublicationSuffixes(stage, pending, data)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := equalRecord(stage, data); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := equalRecord(pending, data); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := equalRecord(stage, data); os.IsNotExist(err) {
			file, err := os.OpenFile(stage, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, mode)
			if err != nil {
				return err
			}
			if err = file.Chmod(mode); err == nil {
				_, err = file.Write(data)
			}
			err = errors.Join(err, syncClose(file))
			if err != nil {
				return err
			}
		}
	}
	if err := equalRecord(pending, data); os.IsNotExist(err) {
		if err := os.Rename(stage, pending); err != nil {
			return err
		}
		if err := syncDirectory(filepath.Dir(path)); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if err := os.Link(pending, path); os.IsExist(err) {
		if err = equalRecord(path, data); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if err := syncDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	return consumePublicationSuffixes(stage, pending, data)
}

func publishReplace(path string, data []byte, mode os.FileMode) error {
	current, currentMode, exists, err := readOptionalRegular(path)
	if err != nil {
		return err
	}
	stage, pending := path+".stage", path+".pending"
	if exists && bytes.Equal(current, data) {
		return consumePublicationSuffixes(stage, pending, data)
	}
	if pendingData, _, pendingExists, pendingErr := readOptionalRegular(pending); pendingErr != nil {
		return pendingErr
	} else if pendingExists && !bytes.Equal(pendingData, data) {
		if err := removePublicationFile(pending); err != nil {
			return err
		}
	}
	if _, _, pendingExists, err := readOptionalRegular(pending); err != nil {
		return err
	} else if !pendingExists {
		if !exists {
			currentMode = mode
		}
		if err := writeTaskPending(pending, data, currentMode); err != nil {
			return err
		}
	}
	actual, _, actualExists, err := readOptionalRegular(path)
	if err != nil || actualExists != exists || exists && !bytes.Equal(actual, current) {
		return errors.Join(err, errors.New("record changed before replacement"))
	}
	if err := os.Rename(pending, path); err != nil {
		return err
	}
	if err := removePublicationFile(stage); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}

func consumePublicationSuffixes(stage, pending string, data []byte) error {
	for _, path := range []string{stage, pending} {
		if err := equalRecord(path, data); err == nil {
			if removeErr := removePublicationFile(path); removeErr != nil {
				return removeErr
			}
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return syncDirectory(filepath.Dir(stage))
}

func consumeFile(path string) error {
	return errors.Join(os.Remove(path), syncDirectory(filepath.Dir(path)))
}

func removePublicationFile(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || !info.Mode().IsRegular() {
		return errors.Join(err, fmt.Errorf("invalid publication stage: %s", path))
	}
	return os.Remove(path)
}

func equalRecord(path string, expected []byte) error {
	actual, err := readRegularNoFollow(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(actual, expected) {
		return fmt.Errorf("record differs:%s", path)
	}
	return nil
}

func ensureDirectoryNoFollow(path string, mode os.FileMode) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("invalid directory path: %s", path)
	}
	missing := make([]string, 0)
	for current := path; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
				return fmt.Errorf("invalid component: %s", current)
			}
			break
		}
		if !os.IsNotExist(err) {
			return err
		}
		missing = append(missing, current)
		if filepath.Dir(current) == current {
			return fmt.Errorf("missing ancestor: %s", path)
		}
	}
	for index := len(missing) - 1; index >= 0; index-- {
		if err := os.Mkdir(missing[index], mode); err != nil {
			return err
		}
		if err := syncDirectory(filepath.Dir(missing[index])); err != nil {
			return err
		}
		info, err := os.Lstat(missing[index])
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("invalid created directory: %s", missing[index])
		}
	}
	return nil
}
