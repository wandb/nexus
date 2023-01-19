"""nexus"""

def get_nexus_path():
    base = Path(__file__).parent
    path = (base / f"bin" / "wandb-nexus").resolve()
    return path
