#define _POSIX_C_SOURCE 200809L
#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#ifndef _WIN32
#include <unistd.h>
#endif
#ifndef BUFFER_MODE
#define BUFFER_MODE -1
#endif

int main(void) {
    if (BUFFER_MODE >= 0 && setvbuf(stdout, NULL, BUFFER_MODE, BUFSIZ) != 0)
        return 4;
    errno = 0;
    int a = printf("A");
    int b = putchar('B');
    int c = printf("\r\n中\n");
    int d = printf("C");
    int e = fflush(stdout);
    fprintf(stderr, "results:%d,%d,%d,%d,%d errno=%d\n", a,b,c,d,e,errno);
#ifndef _WIN32
    fprintf(stderr, "isatty:%d\n", isatty(fileno(stdout)));
#endif
    if (getchar() == 'x') {
        printf("pending");
        abort();
    }
    if (getchar() == 'y') {
        for (int i = 0; i < 100; ++i)
            puts("123456789");
    }
    return 0;
}
