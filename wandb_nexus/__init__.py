"""nexus"""

from pathlib import Path


def get_nexus_path():
    base = Path(__file__).parent
    goos = platform.system().lower()
    goarch = platform.machine().lower()
    path = (base / f"bin-{goos}-{goarch}" / "wandb-nexus").resolve()
    return path
