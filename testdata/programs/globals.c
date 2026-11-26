int grid[1000][2];
static int total;

static void visit(int depth) {
    static int visits;
    visits++;
    total += visits;
    grid[900][1] = total;
    if (depth > 0)
        visit(depth - 1);
}

int main(void) {
    visit(2);
    return grid[900][1] == 6 ? 0 : 1;
}
