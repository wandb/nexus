import pytest
import wandb


@pytest.fixture(scope="session")
def wandb_require_nexus():
    wandb.require(experiment="nexus")
