include("libwbnexus.jl")
import .libwbnexus

run = libwbnexus.nexus_start()
libwbnexus.nexus_finish(run)
