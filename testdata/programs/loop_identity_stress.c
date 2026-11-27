struct pair {
    int a;
    int b;
};

int main(void) {
    int sum = 0;
    int data[32];
    struct pair pairs[8];
    for (int i = 0; i < 400; i++) {
        int x = i * 2;
        int y = x + 1;
        data[i % 32] = x;
        pairs[i % 8].a = x;
        pairs[i % 8].b = y;
        sum += x + y + data[i % 32] + pairs[i % 8].a + pairs[i % 8].b;
    }
    return sum == 0 ? 0 : 1;
}
