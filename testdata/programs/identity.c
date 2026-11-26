static int recurse(int depth) {
    int value = depth;
    if (depth)
        return value + recurse(depth - 1);
    return value;
}

int main(void) {
    int answer = 0;
    int shadow = 10;
    for (int i = 0; i < 2; i++) {
        int iteration = i;
        int shadow = iteration + 1;
        answer += shadow; /* loop sample */
    }
    answer += recurse(2);
    return (answer == 6 && shadow == 10) ? 0 : 1;
}
