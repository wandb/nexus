"""nexus"""

def get_nexus_path(self):
    base = Path(__file__).parent
    path = (base / f"bin" / "wandb-nexus").resolve()
    return path
