#include <stdio.h>

static int helper_loop(void) {
    int s = 0;
    for (int h = 0; h < 2; h++) {
        s += h;
    }
    return s;
}

int main(void) {
    int answer = 0;

    for (int i = 0; i < 3; i++) {
        int x = i * 2;
        answer += x + i;
    }
    for (int i = 0; i < 2; i++) {
        answer += i;
    }
    for (int i = 10; i < 12; i++) {
        answer += i;
    }

    for (int outer = 0; outer < 2; outer++) {
        for (int inner = 0; inner < 2; inner++) {
            answer += outer * 10 + inner;
        }
    }

    for (int shadow = 0; shadow < 2; shadow++) {
        for (int shadow = 0; shadow < 1; shadow++) {
            answer += shadow;
        }
    }

    for (int a = 0, b = 0; a < 2; a++) {
        answer += a + b;
    }

    int m;
    for (m = 0; m < 2; m++) {
        answer += m;
    }

    int k = 0;
    while (k < 2) {
        int y = k;
        answer += y;
        k++;
    }

    if (answer > 0) {
        for (int q = 0; q < 2; q++) {
            answer += q;
        }
    }

    int w = 0;
    while (w < 2) {
        for (int z = 0; z < 2; z++) {
            answer += z;
        }
        w++;
    }

    for (int c = 0; c < 3; c++) {
        if (c == 1)
            continue;
        answer += c;
    }

    answer += helper_loop();
    printf("%d\n", answer);
    return 0;
}
