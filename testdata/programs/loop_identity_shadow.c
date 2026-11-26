int main(void) {
    int i = 100;
    int answer = 0;
    for (int i = 0; i < 2; i++) {
        answer += i;
    }
    for (int i = 0; i < 2; i++) {
        int i = 5;
        answer += i;
    }
    for (int j = 0; j < 2; j++) {
        answer += j;
        {
            int j = 7;
            answer += j;
        }
    }
    return 0;
}
