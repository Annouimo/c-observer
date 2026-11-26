"""Read-only DWARF capture, loaded inside GDB by the MI transport."""
import gdb
import json
import os
import re

_opts = {}
_visible = set()
_entry_caller = None
_started = False


def _path(path):
    if not path:
        return ""
    paths = [path]
    if os.name == "nt" and not path.isascii():
        # Some GCC builds write UTF-8 DWARF names which GDB decodes as ANSI.
        try:
            paths.append(path.encode("mbcs").decode("utf-8"))
        except UnicodeError:
            pass
    for candidate in paths:
        absolute = os.path.normpath(os.path.join(_opts["Directory"], candidate))
        for key in (candidate, absolute, os.path.basename(candidate)):
            mapped = (_opts["Map"] or {}).get(key)
            if mapped:
                return os.path.normpath(os.path.join(_opts["Directory"], mapped))
        if absolute in _visible:
            return absolute
    return os.path.normpath(os.path.join(_opts["Directory"], path))



def _windows_function_breakpoints():
    # On native Windows, rbreak can return no locations for a relative DWARF
    # filename even though GDB has the source's function symbols.
    visible = False
    count = 0
    for line in gdb.execute("info functions", to_string=True).splitlines():
        if line.startswith("File ") and line.endswith(":"):
            visible = _path(line[5:-1]) in _visible
        elif line == "Non-debugging symbols:":
            break
        elif visible:
            match = re.match(r"^\d+:\s+.*?\b([A-Za-z_][A-Za-z_0-9]*)\s*\(", line)
            if match and match[1] != "main":
                symbol = gdb.lookup_static_symbol(match[1]) or gdb.lookup_global_symbol(match[1])
                if symbol and symbol.symtab and _path(symbol.symtab.fullname()) in _visible:
                    gdb.Breakpoint(function=match[1], internal=True)
                    count += 1
                    if count >= 255:
                        break

def observer_init(options):
    global _opts, _visible
    _opts = json.loads(options)
    _visible = set(_opts["Files"] or [])
    main = gdb.lookup_global_symbol("main")
    if main is None or main.symtab is None:
        raise gdb.GdbError("No C main function with debug information")
    if _opts["Source"] in _visible:
        try:
            if os.name == "nt":
                gdb.Breakpoint("main", internal=True)
                _windows_function_breakpoints()
            elif not gdb.rbreak(".*", minsyms=False, throttle=256, symtabs=[main.symtab]):
                gdb.Breakpoint("main", internal=True)
        except (gdb.error, RuntimeError):
            gdb.Breakpoint("main", internal=True)
    else:
        gdb.Breakpoint("main", internal=True)


def _location(frame):
    sal = frame.find_sal()
    raw = sal.symtab.fullname() if sal.symtab else ""
    path = _path(raw)
    return dict(file=path if path in _visible else "", reported_file=raw,
                line=sal.line, function=frame.name() or "")


def _decl(symbol):
    return (dict(file=_path(symbol.symtab.fullname()), line=symbol.line)
            if symbol.symtab and symbol.line > 0 else None)


def _array_page(path, decl, total, budget):
    choices = [r for r in reversed(_opts["Ranges"] or []) if r["Path"] == path]
    default = None
    chosen = None
    for rule in choices:
        target = rule["Declaration"]
        if target and decl and target.get("file") == decl["file"] and target.get("line") == decl["line"]:
            chosen = rule
            break
        if target is None and default is None:
            default = rule
    rule = chosen or default
    start = rule["Start"] if rule else 0
    count = rule["Count"] if rule else _opts["Array"]
    return start, max(0, min(count, _opts["Array"], total - start, budget[0]))


def _value(name, path, value, identity, reliable, budget, depth, decl, index=None):
    budget[0] -= 1
    obj = dict(id=identity, name=name, identity_reliable=reliable,
               status="observed", kind="scalar", children_complete=True, type="")
    if index is not None:
        obj["index"] = index
    if value is None or value.is_optimized_out:
        obj["status"] = "unavailable"
        return obj
    typ = value.type.strip_typedefs()
    code = typ.code
    obj["type"] = str(value.type)
    try:
        if code == gdb.TYPE_CODE_ARRAY:
            low, high = typ.range()
            count = max(0, high - low + 1)
            obj.update(kind="array", array_length=count, children_complete=False)
            target = typ.target().strip_typedefs()
            if str(target) == "char":
                obj["kind"] = "string"
                limit = min(count, 128)
                data = bytes(int(value[key]) & 255 for key in range(low, low + limit))
                prefix, separator, _ = data.partition(b"\0")
                obj["value"] = json.dumps(prefix.decode("utf-8", "replace"), ensure_ascii=False)
                if not separator and limit < count:
                    obj["value_truncated"] = True
            else:
                obj["value"] = "[" + str(count) + "]"
            if depth < _opts["Depth"]:
                start, length = _array_page(path, decl, count, budget)
                children = []
                for offset in range(start, start + length):
                    if budget[0] <= 0:
                        break
                    suffix = "[" + str(offset) + "]"
                    children.append(_value(suffix, path + suffix, value[low + offset],
                                           identity + suffix, reliable, budget, depth + 1, decl, offset))
                obj.update(children=children, children_complete=start == 0 and len(children) == count)
        elif code in (gdb.TYPE_CODE_STRUCT, gdb.TYPE_CODE_UNION):
            obj.update(kind="struct", children_complete=False)
            if depth < _opts["Depth"]:
                fields = typ.fields()
                children = []
                counts = {}
                for field in fields:
                    counts[field.name] = counts.get(field.name, 0) + 1
                for offset, field in enumerate(fields):
                    if budget[0] <= 0:
                        break
                    if not field.name:
                        continue
                    suffix = "." + field.name
                    unique = counts[field.name] == 1
                    member_id = identity + (suffix if unique else "/field-" + str(offset))
                    children.append(_value(field.name, path + suffix, value[field], member_id,
                                           reliable and unique, budget, depth + 1, decl))
                obj.update(children=children, children_complete=len(children) == len(fields))
        else:
            if code == gdb.TYPE_CODE_PTR:
                obj["kind"] = "pointer"
            text = value.format_string(raw=True, symbols=False)
            obj["value"] = text[:1024]
            if len(text) > 1024:
                obj["value_truncated"] = True
    except (gdb.error, ValueError, OverflowError):
        obj["status"] = "unavailable"
    return obj


def _read(symbol, frame):
    try:
        return symbol.value(frame)
    except (gdb.error, ValueError):
        return None


def _blocks(frame):
    try:
        block = frame.block()
    except gdb.error:
        return []
    blocks = []
    while block is not None:
        blocks.append(block)
        block = block.superblock
    return blocks


def observer_capture():
    global _entry_caller, _started
    frames = []
    frame = gdb.newest_frame()
    while frame is not None and len(frames) < 32:
        frames.append(frame)
        frame = frame.older()
    locations = [_location(f) for f in frames]
    entry = None
    if frame is None:
        for depth in reversed(range(len(frames) - 1)):
            loc = locations[depth]
            if loc["function"] == "main" and loc["file"] == _opts["Source"]:
                caller = frames[depth + 1].pc()
                if not _started and depth == 0:
                    _entry_caller = caller
                if caller == _entry_caller:
                    entry = depth
                break
    _started = True
    result = dict(location=locations[0] if locations else {}, frames=[], globals=[])
    budget = [_opts["Nodes"]]
    static_ids = set()
    for depth, (frame, location) in enumerate(zip(frames, locations)):
        reliable_frame = depth == entry
        frame_id = "entry-main" if reliable_frame else "frame-" + str(depth)
        frame_info = dict(id=frame_id, identity_reliable=reliable_frame, location=location,
                          variables=[], variables_complete=False)
        result["frames"].append(frame_info)
        if not location["file"]:
            continue
        blocks = _blocks(frame)
        frame_info["variables_complete"] = bool(blocks)
        for block_no, block in enumerate(blocks):
            for symbol in block:
                if not symbol.is_variable and not symbol.is_argument:
                    continue
                decl = _decl(symbol)
                if not symbol.print_name or not decl or decl["file"] not in _visible:
                    continue
                if budget[0] <= 0:
                    frame_info["variables_complete"] = False
                    break
                name = symbol.print_name
                if block.is_global or block.is_static or symbol.addr_class == gdb.SYMBOL_LOC_STATIC:
                    scope = ("global" if block.is_global else
                             "file_static" if block.is_static else "function_static")
                    identity = "static:" + decl["file"] + ":" + str(decl["line"]) + ":" + name
                    if scope == "function_static":
                        identity += ":" + str(block.start)
                    if identity not in static_ids:
                        static_ids.add(identity)
                        value = _value(name, name, _read(symbol, frame), identity, True, budget, 0, decl)
                        value.update(scope=scope, declaration=decl)
                        result["globals"].append(value)
                    continue
                value = _read(symbol, frame)
                typ = value.type.strip_typedefs() if value is not None else None
                reliable = (reliable_frame and block.function is not None and
                            (symbol.is_argument or typ is not None and not typ.dynamic))
                identity = frame_id + ":" + str(block_no) + ":" + str(len(frame_info["variables"]))
                if reliable:
                    identity = frame_id + ":" + str(decl["line"]) + ":" + name
                item = _value(name, name, value, identity, reliable, budget, 0, decl)
                item.update(scope="parameter" if symbol.is_argument else "local", declaration=decl)
                frame_info["variables"].append(item)
    print("COBS:" + json.dumps(result, separators=(",", ":")))
