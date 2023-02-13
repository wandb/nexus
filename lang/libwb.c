


#include "libwb.h"
#include "../nexus/libwbnexus.h"


int wandb_init(wandb_run *run) {
    int n = nexus_start();
    run->num = n;
    return 0;
}

void wandb_history_clear(wandb_history *history) {
}

void wandb_history_add_float(wandb_history *history, char *key, float value) {
}

void wandb_history_step(wandb_history *history, int step) {
}

void wandb_log(wandb_run *run, wandb_history *hist) {
    int num = run->num;
    nexus_log(num);
}

void wandb_finish(wandb_run *run) {
    int num = run->num;
    nexus_finish(num);
}
