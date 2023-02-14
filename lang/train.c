#include <stdio.h>
#include <unistd.h>
#include <assert.h>
#include "../nexus/libwbnexus.h"
#include "libwb.h"


int main(int argc, char **argv) {
    int i;
    int x;
    int n;
    int rc;

    wandb_run run;
    wandb_history history;

    rc = wandb_init(&run);
    assert(rc == 0);
    for (i=0; i < 10; i++) {
        wandb_history_clear(&history);
        wandb_history_add_float(&history, "num", i);
        wandb_log(&run, &history);
    }
    wandb_finish(&run);
    return 0;
}
