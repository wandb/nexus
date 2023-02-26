#!/usr/bin/env julia

include("libwbnexus.jl")
import .libwbnexus

run = libwbnexus.nexus_start()
for i in 1:5
    d = Float64(i)
    println(i, " ", d)
    libwbnexus.nexus_log_scaler(run, "data", Float32(i))
end
libwbnexus.nexus_finish(run)
