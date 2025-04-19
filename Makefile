.PHONY: all build run test clean lint dev-install ngrok tunnel

# Default target
all: tunnel

# Delegate all targets to the Go project's Makefile
%:
	@cd go-brain && $(MAKE) $@

VENV = .venv
PYTHON = $(VENV)/bin/python
PIP = $(VENV)/bin/pip

# Create virtual environment
venv:
	python3 -m venv $(VENV)
	$(PIP) install --upgrade pip

# Install production dependencies
install: venv
	$(PIP) install -r requirements.txt

# Install development dependencies
dev-install: venv
	$(PIP) install -r requirements-dev.txt

# Run tests with coverage
test:
	$(PYTHON) -m pytest

# Run linters
lint:
	$(PYTHON) -m flake8 .
	$(PYTHON) -m mypy .
	$(PYTHON) -m isort --check-only .
	$(PYTHON) -m black --check .

# Format code
format:
	$(PYTHON) -m isort .
	$(PYTHON) -m black .

# Clean build artifacts and virtual environment
clean:
	rm -rf build/
	rm -rf dist/
	rm -rf *.egg-info/
	rm -rf .pytest_cache/
	rm -rf .coverage
	rm -rf $(VENV)
	find . -type d -name "__pycache__" -exec rm -rf {} +
	find . -type f -name "*.pyc" -delete

# Run the application
run:
	$(PYTHON) beebrain_app.py

# Activate virtual environment (for shell use)
activate:
	@echo "To activate the virtual environment, run:"
	@echo "source $(VENV)/bin/activate" 