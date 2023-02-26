#!/usr/bin/env julia

include("wandb.jl")

run = wandb.init()

for i in 1:5
    d = Float64(i)
    println(i, " ", d)
    wandb.log_scaler(run, "data", Float32(i))
end

wandb.finish(run)
