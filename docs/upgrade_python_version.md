# How to Upgrade Python Version in a Poetry Project Using Pyenv

## Steps to Upgrade Python Version

### 1. Verify the Current Python Version

First, check the current Python version used by your project. You can do this by running:

```bash
poetry run python --version
```

If this shows an older Python version, you’ll want to upgrade it.

### 2. Install the New Python Version Using Pyenv

Use `pyenv` to install the desired Python version. For example, to install Python 3.12.6:

```bash
pyenv install 3.12.6
```

You can verify that the new version was installed by running:

```bash
pyenv versions
```

### 3. Set the Local Python Version

Set Python 3.12.6 as the local version for your project directory:

```bash
pyenv local 3.12.6
```

This will create a `.python-version` file in the root of your project, locking the directory to use Python 3.12.6.

Update `pyproject.toml`

### 4. Recreate the Poetry Virtual Environment

Now, instruct Poetry to use the newly installed Python version. First, remove the existing virtual environment (if it exists):

```bash
poetry env remove python
```

Then, create a new virtual environment with the updated Python version:

```bash
poetry env use python3.12
```

> If you see an error like `Could not find the python executable python3.12.6`, ensure that the new Python version is available in your shell by running `pyenv which python3.12`.

### 5. Verify the Python Version in the New Environment

Check that Poetry is using the correct Python version:

```bash
poetry run python --version
```

This should now return `Python 3.12.x`.

### 6. Reinstall Project Dependencies

Since you've switched Python versions, you'll need to reinstall all project dependencies:

```bash
poetry install
```

This will recreate the virtual environment and install all dependencies with the new Python version.

### 7. Confirm Everything is Working

To confirm that your project is now running with the new Python version, you can run your tests or start your project normally:

```bash
poetry run <your-script>
```

---

## Troubleshooting

### Could not find the python executable python3.12.6

This error indicates that Poetry is unable to find the specified Python version. To resolve this:

- Ensure Python 3.12.6 is installed using `pyenv install 3.12.6`.
- Verify that `pyenv` is correctly managing your Python versions with `pyenv versions`.
- Check the path for the Python interpreter by running `pyenv which python3.12` and use that path in the `poetry env use` command.

```bash
poetry env use $(pyenv which python3.12)
```

---

By following these steps, you should be able to upgrade the Python version for your Poetry project using `pyenv`.
