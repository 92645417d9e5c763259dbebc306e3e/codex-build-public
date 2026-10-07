import base64
import json
import subprocess
import tempfile
from pathlib import Path

GENERATOR = Path("app-server-protocol/scripts/write_schema_fixtures.py")
SCHEMA_ROOT = Path("app-server-protocol/schema")
OUTPUTS = ("json", "typescript", "precomputed")
def generate(root):
    command = ["/usr/bin/python3", "-s", str(GENERATOR), "--schema-root", str(root)]
    subprocess.run(command, check=True, stdout=subprocess.DEVNULL)
    subprocess.run([*command, "--experimental"], check=True, stdout=subprocess.DEVNULL)
    try:
        return [
            {"ContentBase64": base64.b64encode(path.read_bytes()).decode("ascii"), "Path": path.relative_to(root).as_posix()}
            for name in OUTPUTS
            for path in sorted((root / name).rglob("*"))
            if path.is_file()
        ]
    except OSError as error:
        raise RuntimeError("SchemaPayloadTransportFailure") from error


with tempfile.TemporaryDirectory() as first, tempfile.TemporaryDirectory() as second:
    stable = generate(Path(first))
    repeated = generate(Path(second))
    if stable != repeated:
        raise RuntimeError("SchemaPostimagesDiffer")
    print(json.dumps({"Files": stable}, ensure_ascii=False, separators=(",", ":"), sort_keys=True))
