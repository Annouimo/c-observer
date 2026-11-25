#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

struct birthday {
    char name[10];
    int day;
};

static int compare(const void *left, const void *right) {
    int a = *(const int *)left;
    int b = *(const int *)right;
    return (a > b) - (a < b);
}

int main(void) {
    char mode[8];
    int seed;
    if (scanf("%7s%d", mode, &seed) != 2)
        return 2;
    if (!strcmp(mode, "crash"))
        abort();
    if (!strcmp(mode, "spin")) {
        FILE *ready = fopen("inferior.pid", "w");
        if (!ready)
            return 3;
        fprintf(ready, "%ld\n", (long)getpid());
        fclose(ready);
        for (;;) {}
    }
    struct birthday birthdays[10010] = {{0}};
    strcpy(birthdays[9000].name, "Ada");
    birthdays[9000].day = seed;
    birthdays[9000].day++;
    int scores[] = {3, 1, 2};
    qsort(scores, 3, sizeof scores[0], compare);
    printf("%s:%d:%d\n", birthdays[9000].name, birthdays[9000].day, scores[0]);
    fprintf(stderr, "done\n");
    return 0;
}
