#!/usr/bin/env python3
"""Parse executable shell commands in Markdown documentation contracts."""

import re
import sys


SHELL_FENCE = re.compile(r"^\s*```(?:bash|sh|shell|zsh)\s*$", re.IGNORECASE)
CLOSING_FENCE = re.compile(r"^\s*```\s*$")
ASSIGNMENT = re.compile(r"[A-Za-z_][A-Za-z0-9_]*=")
ESCAPED_COMMAND_QUOTE = re.compile(
    r"(?<!\S)(?:-c|--command)(?:[ \t]+|=)?\\[\"']"
)
UNSUPPORTED_PSQL = object()


def static_word_value(word):
    """Return a shell word's value when it is statically knowable."""
    if len(word) >= 2 and word[0] in "\"'" and word[-1] == word[0]:
        return word[1:-1]
    return word


def has_continuation(line):
    stripped = line.rstrip()
    return (len(stripped) - len(stripped.rstrip("\\"))) % 2 == 1


def shell_fence_commands(path):
    commands = []
    in_shell_fence = False
    block = 0
    pending = None
    pending_line = None

    with open(path, encoding="utf-8") as source:
        for line_number, raw_line in enumerate(source, 1):
            line = raw_line.rstrip("\n")
            if not in_shell_fence:
                if SHELL_FENCE.match(line):
                    in_shell_fence = True
                    block += 1
                continue
            if CLOSING_FENCE.match(line):
                if pending is not None:
                    commands.append((block, pending_line, pending))
                pending = None
                pending_line = None
                in_shell_fence = False
                continue

            stripped = line.strip()
            if pending is None:
                if not stripped or stripped.startswith("#"):
                    continue
                pending = line
                pending_line = line_number
            else:
                pending += "\n" + line
            if not has_continuation(line):
                commands.append((block, pending_line, pending))
                pending = None
                pending_line = None

    return commands


def split_shell_operators(command):
    parts = []
    start = 0
    quote = None
    escaped = False
    index = 0
    while index < len(command):
        char = command[index]
        if escaped:
            escaped = False
        elif char == "\\":
            escaped = True
        elif quote:
            if char == quote:
                quote = None
        elif char in "\"'":
            quote = char
        elif char in ";|&":
            parts.append(command[start:index])
            if index + 1 < len(command) and command[index + 1] == char and char in "|&":
                index += 1
            start = index + 1
        index += 1
    parts.append(command[start:])
    return parts


def shell_words(command):
    command = command.replace("\\\n", "")
    words = []
    current = []
    quoted = False
    quote = None
    escaped = False
    for char in command:
        if escaped:
            if char != "\n":
                current.append(char)
            escaped = False
        elif char == "\\":
            current.append(char)
            escaped = True
        elif quote:
            current.append(char)
            if char == quote:
                quote = None
        elif char in "\"'":
            current.append(char)
            quoted = True
            quote = char
        elif char.isspace():
            if current:
                words.append(("".join(current), quoted))
                current = []
                quoted = False
        else:
            current.append(char)
    if current:
        words.append(("".join(current), quoted))
    return words


def consume_options(words, index, options_with_value, strict=False):
    while index < len(words) and not words[index][1] and words[index][0].startswith("-"):
        option = words[index][0]
        if strict and option not in options_with_value:
            return None
        index += 1
        if option in options_with_value and index < len(words):
            index += 1
    return index


DOCKER_OPTIONS_WITH_VALUE = {
    "--config",
    "--context",
    "--host",
    "-H",
    "-c",
}
COMPOSE_OPTIONS_WITH_VALUE = {
    "--ansi",
    "--env-file",
    "--file",
    "--parallel",
    "--profile",
    "--progress",
    "--project-directory",
    "-p",
    "--project-name",
    "-f",
}
COMPOSE_BOOLEAN_OPTIONS = {
    "--all-resources",
    "--compatibility",
    "--dry-run",
    "--help",
    "--no-ansi",
    "--verbose",
}
COMPOSE_EXEC_OPTIONS_WITH_VALUE = {
    "--detach-keys",
    "--env",
    "--user",
    "--workdir",
    "-e",
    "-u",
    "-w",
}
COMPOSE_EXEC_BOOLEAN_OPTIONS = {"-T"}


def consume_compose_exec_options(words, index):
    """Consume the documented options for ``docker compose exec``."""
    while index < len(words) and not words[index][1] and words[index][0].startswith("-"):
        option = words[index][0]
        if option in COMPOSE_EXEC_BOOLEAN_OPTIONS:
            index += 1
            continue
        if option in COMPOSE_EXEC_OPTIONS_WITH_VALUE:
            index += 1
            if index >= len(words):
                return None
            index += 1
            continue
        return None
    return index


def consume_compose_options(words, index):
    """Consume known docker compose options preceding its subcommand."""
    while index < len(words) and not words[index][1] and words[index][0].startswith("-"):
        option = words[index][0]
        if option in COMPOSE_BOOLEAN_OPTIONS:
            index += 1
            continue
        if option in COMPOSE_OPTIONS_WITH_VALUE:
            index += 1
            if index >= len(words):
                return None
            index += 1
            continue
        if option.startswith("--") and any(
            option.startswith(f"{known}=") for known in COMPOSE_OPTIONS_WITH_VALUE if known.startswith("--")
        ):
            index += 1
            continue
        if option.startswith(("-f", "-p")) and option not in {"-f", "-p"}:
            index += 1
            continue
        return None
    return index


def contains_unquoted_word(words, expected):
    return any(word == expected and not quoted for word, quoted in words)


def is_assignment(word):
    return bool(ASSIGNMENT.match(word))


def is_assignment_word(word, quoted):
    # A quoted value still makes the complete token appear quoted to the
    # tokenizer, but the assignment name itself remains shell syntax.
    return not word.startswith(("'", '"')) and is_assignment(word)


def psql_words(segment):
    words = shell_words(segment)
    index = 0
    while index < len(words) and is_assignment_word(*words[index]):
        assignment = words[index][0]
        command_start = assignment.find("$(")
        if command_start >= 0 and (command_start == 0 or assignment[command_start - 1] != "'"):
            command = assignment[command_start + 2 :]
            if command:
                words[index] = (command, False)
                break
            index += 1
            break
        index += 1

    while index < len(words):
        word, quoted = words[index]
        if quoted:
            return None
        if word == "psql":
            return [item for item, _ in words[index:]]
        if word == "sudo":
            index = consume_options(words, index + 1, {"-u", "-g", "-h", "-p", "-r", "-t", "-C", "--user", "--group", "--host", "--prompt", "--role", "--type", "--close-from"})
        elif word == "env":
            index = consume_options(words, index + 1, {"-u", "--unset", "-C", "--chdir"})
            while index < len(words) and is_assignment_word(*words[index]):
                index += 1
        elif word == "command":
            index = consume_options(words, index + 1, {"-p", "-v", "-V"})
        elif word == "docker":
            docker_index = consume_options(words, index + 1, DOCKER_OPTIONS_WITH_VALUE, strict=True)
            if docker_index is None:
                return UNSUPPORTED_PSQL if contains_unquoted_word(words[index + 1 :], "psql") else None
            index = docker_index
            if index < len(words) and words[index] == ("compose", False):
                index += 1
                compose_index = consume_compose_options(words, index)
                if compose_index is None:
                    return UNSUPPORTED_PSQL if contains_unquoted_word(words[index:], "psql") else None
                index = compose_index
            if index >= len(words) or words[index] != ("exec", False):
                return UNSUPPORTED_PSQL if contains_unquoted_word(words[index:], "psql") else None
            exec_index = consume_compose_exec_options(words, index + 1)
            if exec_index is None:
                return UNSUPPORTED_PSQL if contains_unquoted_word(words[index + 1 :], "psql") else None
            index = exec_index
            if index >= len(words):
                return None
            index += 1  # container
        else:
            return UNSUPPORTED_PSQL if contains_unquoted_word(words[index:], "psql") else None
    return None


def port_error(words):
    ports = []
    index = 1
    while index < len(words):
        word = words[index]
        if word in {"-p", "--port"}:
            if index + 1 >= len(words):
                return "missing"
            ports.append(static_word_value(words[index + 1]))
            index += 2
            continue
        if word.startswith("--port="):
            ports.append(static_word_value(word.split("=", 1)[1]))
        elif word.startswith("-p") and word != "-p":
            ports.append(static_word_value(word[2:]))
        index += 1
    if not ports or any(port != "55432" for port in ports):
        return "wrong or missing"
    return None


def validate_psql(paths):
    for path in paths:
        invocation_count = 0
        for _, line_number, command in shell_fence_commands(path):
            for segment in split_shell_operators(command):
                words = psql_words(segment)
                if words is UNSUPPORTED_PSQL:
                    raise SystemExit(f"unsupported wrapper syntax around psql in {path}:{line_number}")
                if words is None:
                    continue
                invocation_count += 1
                if port_error(words):
                    raise SystemExit(f"psql invocation lacks only '-p 55432' in {path}:{line_number}")
                if ESCAPED_COMMAND_QUOTE.search(segment):
                    raise SystemExit(f"psql invocation uses an escaped -c quote in {path}:{line_number}")
        if invocation_count == 0:
            raise SystemExit(f"missing executable psql invocation in {path}")


def starts_with_unquoted(words, expected):
    return len(words) >= len(expected) and all(
        word == value and not quoted
        for (word, quoted), value in zip(words, expected)
    )


def matches_api_event(segment, event):
    words = shell_words(segment)
    expected = {
        "stop": ["pm2", "stop", "sumweave-api"],
        "reset": ["docker", "compose", "down", "-v"],
        "bootstrap": ["make", "postgres-bootstrap"],
        "api_start": ["go", "run", "./cmd/sumweave", "start", "--env", "local"],
        "worker": ["go", "run", "./cmd/sumweave", "jobs", "worker", "--once", "--env", "local"],
        "restart": ["pm2", "start", "ecosystem.config.js"],
    }[event]
    return starts_with_unquoted(words, expected)


def command_events(path, event):
    events = []
    command_index = 0
    for block, line_number, command in shell_fence_commands(path):
        for segment in split_shell_operators(command):
            if matches_api_event(segment, event):
                events.append((command_index, block, line_number))
            command_index += 1
    return events


def validate_api(paths):
    event_names = ("stop", "reset", "bootstrap", "api_start", "worker", "restart")
    for path in paths:
        events = {name: command_events(path, name) for name in event_names}
        stop, reset, bootstrap = events["stop"], events["reset"], events["bootstrap"]
        if not stop:
            raise SystemExit(f"missing executable PM2 stop in {path}")
        if not reset:
            raise SystemExit(f"missing executable volume reset in {path}")
        if not bootstrap:
            raise SystemExit(f"missing executable PostgreSQL bootstrap in {path}")
        if not all(any(s[1] == r[1] and s[0] < r[0] for s in stop) for r in reset):
            raise SystemExit(f"PM2 stop must precede the volume reset in the same shell block: {path}")
        backend_state = "running"
        lifecycle_events = sorted(
            (index, event)
            for event in ("stop", "api_start", "restart", "reset")
            for index, _, _ in events[event]
        )
        for _, event in lifecycle_events:
            if event == "stop":
                backend_state = "stopped"
            elif event == "reset":
                if backend_state != "stopped":
                    raise SystemExit(f"backend must be stopped at every volume reset: {path}")
            else:
                backend_state = "running"
        first_reset = min(reset)[0]
        bootstrap_after_reset = [item for item in bootstrap if item[0] > first_reset]
        if not bootstrap_after_reset:
            raise SystemExit(f"PostgreSQL bootstrap must follow the volume reset in {path}")
        first_bootstrap = min(bootstrap_after_reset)[0]
        if not any(start[0] > first_bootstrap for start in events["api_start"]):
            raise SystemExit(f"API-only start must follow PostgreSQL bootstrap in {path}")
        if not events["worker"]:
            raise SystemExit(f"missing bounded API-only worker in {path}")
        last_worker = max(events["worker"])[0]
        if not any(start[0] > last_worker for start in events["restart"]):
            raise SystemExit(f"normal PM2 restart must follow the API-only worker workflow in {path}")


if __name__ == "__main__":
    mode, *paths = sys.argv[1:]
    if mode == "psql":
        validate_psql(paths)
    elif mode == "api":
        validate_api(paths)
    else:
        raise SystemExit("usage: documentation-contract-parser.py {psql|api} PATH...")
