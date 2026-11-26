import gdb
f = gdb.newest_frame()
print("FRAME", f.name(), f.find_sal().symtab.fullname() if f.find_sal().symtab else None, f.find_sal().line)
b = f.block()
i = 0
while b is not None:
    print("BLOCK", i, "start", b.start, "end", b.end, "fn", b.function, "global", b.is_global, "static", b.is_static)
    for s in b:
        if s.is_variable or s.is_argument:
            try:
                print("  SYM", s.print_name, "line", s.line, "file", s.symtab.fullname() if s.symtab else None, "arg", s.is_argument, "addr", s.addr_class, "block", b.start)
            except Exception as e:
                print("  SYMERR", e)
    b = b.superblock
    i += 1
