#!/usr/bin/env julia

include("libwbnexus.jl")
import .libwbnexus

run = libwbnexus.nexus_start()
libwbnexus.nexus_finish(run)
