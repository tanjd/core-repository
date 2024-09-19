def bye(name: str) -> str:
    """Return a friendly greeting."""
    message = f"Bye {name}"
    if name == "":
        message = "Bye nobody"
    return message
