#include <stdio.h>
#include <stdlib.h>

struct Birthday {
    char name[24];
    struct Date { int day; } date;
};

int main(void) {
    int n = read_int();
    struct Birthday birthdays[4] = {0};
    for (int i = 1; i <= n; i++) {
        scanf("%23s", birthdays[i].name);
        birthdays[i].date.day = read_int();
    }
    int q = read_int();
    for (int i = 0; i < q; i++) {
        int index = read_int(), x = read_int();
        birthdays[index].date.day += x;
    }
    for (int i = 1; i <= n; i++) {
        printf("%s %d\n", birthdays[i].name, birthdays[i].date.day);
    }
    return 0;
}
