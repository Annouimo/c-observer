int main(void) {
    int n = 0;
    int rounds = 0;
again:
    for (int i = 0; i < 2; i++) {
        n += i;
    }
    rounds++;
    if (rounds < 2)
        goto again;
    return n == 2 ? 0 : 1;
}
