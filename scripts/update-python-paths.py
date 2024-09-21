import json
import os

libs_path = "/workspace/libs"
vscode_settings_path = "/workspace/.vscode/settings.json"

# Get all directories inside libs
libs_dirs = [
    os.path.join(libs_path, name)
    for name in os.listdir(libs_path)
    if os.path.isdir(os.path.join(libs_path, name))
]

# Read existing settings.json
with open(vscode_settings_path, "r") as f:
    settings = json.load(f)

# Update extraPaths
settings["python.analysis.extraPaths"] = libs_dirs
settings["python.autoComplete.extraPaths"] = libs_dirs

# Write the updated settings.json
with open(vscode_settings_path, "w") as f:
    json.dump(settings, f, indent=4)
