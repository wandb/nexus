"""yea setup."""

from setuptools import setup
from distutils.command.install import install


class post_install(install):
    def __init__(self, dist_obj):
        super().__init__(dist_obj)
        print("GOT INIT *************", self, dist_obj)
        print(vars(self))
        # assert False

    def run(self):
        for x in range(40):
            print("RUN1", x)
        install.run(self)
        for x in range(40):
            print("RUN2", x)

setup(
    name="wandb-nexus",
    version="0.0.1.dev1",
    description="Wandb core",
    packages=["wandb_nexus"],
    zip_safe=False,
    include_package_data=True,
    license="MIT license",
    python_requires=">=3.6",
    cmdclass={"install": post_install},
)
