import base64
import json
import os
import re
import stat
import subprocess
import sys
from pathlib import Path


def read_fixture(path):
    if not stat.S_ISREG(path.lstat().st_mode) or path.resolve() != path.absolute():
        raise RuntimeError("ConfigSchemaPayloadFailure: fixture is not a regular source file")
    return path.read_bytes()


if len(sys.argv) != 2 or not re.fullmatch(r"(?:[0-9a-f]{40}|[0-9a-f]{64})", sys.argv[1]):
    raise RuntimeError("ConfigSchemaPayloadFailure: source commit is absent")
fixture = Path("core/config.schema.json")
before = read_fixture(fixture)
environment = {**os.environ, "CARGO_NET_OFFLINE": "true"}
subprocess.run(["/toolchain/bin/just", "write-config-schema"], env=environment, stdout=sys.stderr, check=True)
generated = read_fixture(fixture)
if not isinstance(json.loads(generated), dict):
    raise RuntimeError("ConfigSchemaPayloadFailure: generator output is not a JSON object")
print(json.dumps({
    "SourceHead": sys.argv[1],
    "BeforeBase64": base64.b64encode(before).decode("ascii"),
    "GeneratedBase64": base64.b64encode(generated).decode("ascii"),
}, sort_keys=True, separators=(",", ":")))
