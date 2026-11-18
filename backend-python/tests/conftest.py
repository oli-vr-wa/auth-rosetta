import os

# Fixed test config, set before anything reads settings. The real root .env is never used.
os.environ["DB_USER"] = "test"
os.environ["DB_PASSWORD"] = "test"
os.environ["JWT_KEY"] = "k" * 64