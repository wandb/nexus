test:
	tox

release-tox: dist ## package and upload release
	twine upload dist/*
	rm -fr build/
	rm -fr dist/

release:
	tox -e release

dist: clean ## builds source and wheel package
	python setup.py sdist bdist_wheel
	ls -l dist

clean: clean-build clean-pyc clean-test ## remove all build, test, coverage and Python artifacts

clean-build: ## remove build artifacts
	rm -fr build/
	rm -fr dist/
	rm -fr .eggs/
	find . -name '*.egg-info' -exec rm -fr {} +
	find . -name '*.egg' -exec rm -f {} +

setup-clean:
	rm -fr build/
	rm -fr dist/

clean-pyc: ## remove Python file artifacts
	find . -name '*.pyc' -exec rm -f {} +
	find . -name '*.pyo' -exec rm -f {} +
	find . -name '*~' -exec rm -f {} +
	find . -name '__pycache__' -exec rm -fr {} +

clean-test: ## remove test and coverage artifacts
	rm -f .coverage
	rm -fr htmlcov/

bumpversion-to-dev:
	pip install -qq bump2version==1.0.1
	python ./tools/bumpversion-tool.py --to-dev

bumpversion-from-dev:
	pip install -qq bump2version==1.0.1
	python ./tools/bumpversion-tool.py --from-dev
