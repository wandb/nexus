"""nexus"""

import platform
from pathlib import Path


def get_nexus_path() -> Path:
    base = Path(__file__).parent
    goos = platform.system().lower()
    goarch = platform.machine().lower()
    path = (base / f"bin-{goos}-{goarch}" / "wandb-nexus").resolve()
    return path
