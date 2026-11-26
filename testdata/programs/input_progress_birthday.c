/* Representative course fixture. Build with the C Exam Kit generated header. */
#include "exam.h"

struct Birthday {
    char name[25];
    Date date;
};

int main(void) {
    int n = read_int();
    struct Birthday birthdays[4] = {0};
    for (int i = 1; i <= n; ++i) {
        read_word(birthdays[i].name);
        int y = read_int(), m = read_int(), d = read_int();
        birthdays[i].date = date_new(y, m, d);
    }
    int q = read_int();
    for (int i = 0; i < q; ++i) {
        int index = read_int(), days = read_int();
        birthdays[index].date = date_add_days(birthdays[index].date, days);
    }
    for (int i = 1; i <= n; ++i) {
        print_text(birthdays[i].name, " ");
        date_print(birthdays[i].date, "%Y-%m-%d", "\n");
    }
}
