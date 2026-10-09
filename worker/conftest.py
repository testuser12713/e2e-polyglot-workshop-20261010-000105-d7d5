"""Pytest path setup: make the worker modules importable as top-level modules."""

import os
import sys

sys.path.insert(0, os.path.dirname(__file__))
