#include <stdio.h>
#include <unistd.h>
#include "../nexus/libwbnexus.h"
#include "libwb.h"


int main(int argc, char **argv) {
    int i;
    int x;
    int n;
    int rc;

    wandb_run run;
    wandb_history history;

    printf("init\n");
    rc = wandb_init(&run);
    printf("init2\n");
    for (i=0; i < 10; i++) {
        wandb_history_clear(&history);
        wandb_history_add_float(&history, "num", i);
        printf("log\n");
        wandb_log(&run, &history);
        printf("log2\n");
    }
    // run.log_kv(key, val);
    // run.log_step(step);
    // run.log_commit();
    wandb_finish(&run);

    // n = nexus_connect();
    // PrintInt(42);
    // x = GetInt();
    // printf("GOT: %d\n", x);
    return 0;
}
