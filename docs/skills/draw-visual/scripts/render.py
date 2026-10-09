#!/usr/bin/env python3
"""Render Mermaid or PlantUML sources as terminal text, and keep Markdown embeds current.

    render.py SOURCE                  print the rendered diagram
    render.py --update FILE.md ...    re-render every embedded diagram in those files
    render.py --check FILE.md ...     fail when an embed is stale, missing, or too wide

An embed is a marker comment followed by a fenced block:

    <!-- draw-visual: diagrams/3-topology.mmd -->
    ```text
    ...rendered text, never edited by hand...
    ```

The marker path is relative to the Markdown file.
Render options live in the source's first line, so every re-render matches:

    %% draw-visual: -x 4 -y 1 -p 0        (Mermaid)
    ' draw-visual: ascii                   (PlantUML; default is Unicode)

Standard library only.
"""
import os
import re
import shlex
import subprocess
import sys

HOME = os.environ.get('DRAW_VISUAL_HOME', os.path.expanduser('~/.local/share/draw-visual'))
MAX_WIDTH = int(os.environ.get('DRAW_VISUAL_MAX_WIDTH', '110'))
MARKER = re.compile(r'^<!-- draw-visual: (\S+) -->\s*$')
OPTIONS = re.compile(r"^\s*(?:%%|')\s*draw-visual:\s*(.*)$")


def fail(message):
    sys.exit('render: ' + message)


def source_options(text):
    first = text.split('\n', 1)[0]
    match = OPTIONS.match(first)
    return shlex.split(match.group(1)) if match else []


def render(path):
    if not os.path.exists(path):
        fail(f'source not found: {path}')
    text = open(path, encoding='utf-8').read()
    options = source_options(text)
    if path.endswith(('.mmd', '.mermaid')):
        tool = os.path.join(HOME, 'bin', 'mermaid-ascii')
        if not os.path.exists(tool):
            fail('mermaid-ascii missing; run scripts/setup.sh')
        command = [tool, '-f', path] + options
        result = subprocess.run(command, capture_output=True, text=True)
    elif path.endswith(('.puml', '.plantuml')):
        java = os.path.join(HOME, 'jre', 'bin', 'java')
        jar = os.path.join(HOME, 'plantuml.jar')
        if not (os.path.exists(java) and os.path.exists(jar)):
            fail('PlantUML missing; run scripts/setup.sh --with-plantuml')
        mode = '-ttxt' if 'ascii' in options else '-utxt'
        command = [java, '-Djava.awt.headless=true', '-jar', jar, mode, '-pipe']
        result = subprocess.run(command, input=text, capture_output=True, text=True)
    else:
        fail(f'unknown source type: {path}')
    output = result.stdout
    if result.returncode != 0 or not output.strip() or 'has crashed' in output:
        detail = (result.stderr or output).strip().split('\n')[0]
        fail(f'{path} did not render: {detail}')
    lines = [line.rstrip() for line in output.split('\n')]
    while lines and not lines[-1]:
        lines.pop()
    while lines and not lines[0]:
        lines.pop(0)
    return lines


def process(markdown_path, write):
    base = os.path.dirname(os.path.abspath(markdown_path))
    lines = open(markdown_path, encoding='utf-8').read().split('\n')
    out, problems, index, count = [], [], 0, 0
    while index < len(lines):
        line = lines[index]
        match = MARKER.match(line)
        out.append(line)
        index += 1
        if not match:
            continue
        count += 1
        if index >= len(lines) or not lines[index].startswith('```'):
            fail(f'{markdown_path}:{index}: marker must be followed by a fenced block')
        fence = lines[index]
        end = index + 1
        while end < len(lines) and not lines[end].startswith('```'):
            end += 1
        if end >= len(lines):
            fail(f'{markdown_path}:{index + 1}: unterminated fenced block')
        rendered = render(os.path.join(base, match.group(1)))
        width = max(len(r) for r in rendered)
        if width > MAX_WIDTH:
            problems.append(f'{markdown_path}: {match.group(1)} is {width} columns wide (limit {MAX_WIDTH})')
        if lines[index + 1:end] != rendered:
            if not write:
                problems.append(f'{markdown_path}: {match.group(1)} embed is stale')
        out += [fence] + rendered + [lines[end]]
        index = end + 1
    if write:
        open(markdown_path, 'w', encoding='utf-8').write('\n'.join(out))
    return count, problems


def main(argv):
    if not argv or argv[0] in ('-h', '--help'):
        print(__doc__)
        return 0
    if argv[0] in ('--update', '--check'):
        write = argv[0] == '--update'
        problems = []
        for path in argv[1:]:
            count, found = process(path, write)
            problems += found
            print(f'{path}: {count} embedded diagram(s) ' + ('updated' if write else 'checked'))
        for problem in problems:
            print('render: ' + problem, file=sys.stderr)
        return 1 if problems else 0
    lines = render(argv[0])
    print('\n'.join(lines))
    width = max(len(line) for line in lines)
    print(f'render: {len(lines)} lines, {width} columns', file=sys.stderr)
    return 0


if __name__ == '__main__':
    sys.exit(main(sys.argv[1:]))
