#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#ifdef _WIN32
#include <windows.h>
#else
#include <unistd.h>
#endif

#define LOG(name, value) printf("%s:%d eof=%d error=%d errno=%d\n", name, (int)(value), !!feof(stdin), !!ferror(stdin), errno)

#ifndef MODE
#define MODE 49
#endif

FILE *probe_stdin;

int main(int argc, char **argv) {
    int mode = MODE;
    int a = 0, b = 0, ch = 0, rc;
    char buf[8] = {0};
    (void)argv;
    errno = 0;
    probe_stdin = stdin;
    if (mode == '1' || mode == EOF) {
        rc = scanf("%d", &a);
        LOG("first", rc);
        rc = scanf("%d", &b);
        LOG("second", rc);
        ch = getchar();
        LOG("separator", ch);
        rc = fgets(buf, sizeof buf, stdin) != NULL;
        LOG("line", rc);
        ch = getchar();
        LOG("X", ch);
        rc = ungetc(ch, stdin);
        LOG("put-back", rc);
        ch = getchar();
        LOG("again", ch);
        ch = getchar();
        LOG("newline", ch);
        ch = getchar();
        LOG("EOF", ch);
        printf("values:%d,%d,%s", a, b, buf);
    } else if (mode == 'Y') {
        while (scanf("%d", &a) == 1)
            b += a;
        LOG("sum", b);
    } else if (mode == 'D') {
        while (fgets(buf, sizeof buf, stdin)) {
            b += strlen(buf);
        }
        LOG("bytes", b);
    } else if (mode == 'F') {
        rc = scanf("%d", &a);
        LOG("failure", rc);
        ch = getchar();
        LOG("percent-c-next", ch);
        rc = scanf("%c", buf);
        LOG("percent-c", rc);
        rc = scanf(" %c", buf);
        LOG("spaced-percent-c", rc);
        rc = scanf("%7s", buf);
        LOG("word", rc);
        rc = fread(buf, 1, 2, stdin);
        LOG("fread", rc);
        rc = ungetc('Z', stdin);
        LOG("replacement", rc);
        ch = getchar();
        LOG("replacement-read", ch);
        rc = fseek(stdin, 1, SEEK_SET);
        LOG("seek", rc);
        rewind(stdin);
        ch = getchar();
        LOG("after-rewind", ch);
        FILE *other = tmpfile();
        if (other) {
            rc = fputs("abc", other);
            rewind(other);
            ch = fgetc(other);
            LOG("other-stream", ch);
            fclose(other);
        }
        if (!freopen("replacement.txt", "r", stdin))
            return 7;
        ch = getchar();
        LOG("reopened", ch);
#ifdef __linux__
    } else if (mode == 'P') {
        int ends[2];
        char path[64];
        if (pipe(ends) || write(ends[1], "q", 1) != 1)
            return 8;
        close(ends[1]);
        snprintf(path, sizeof path, "/proc/self/fd/%d", ends[0]);
        if (!freopen(path, "r", stdin))
            return 9;
        close(ends[0]);
        errno = 0;
        LOG("pipe-errno", errno);
        ch = getchar();
        LOG("pipe-read", ch);
#endif
    } else if (mode == 'C') {
        abort();
    } else if (mode == 'L') {
        for (;;) {
            a = !a;
        }
    }
    fprintf(stderr, "done (%d)\n", argc);
    return ferror(stdin) ? 1 : 0;
}
