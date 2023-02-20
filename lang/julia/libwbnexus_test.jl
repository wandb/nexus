include("libwbnexus.jl")
import .libwbnexus
# We run this test code for libtwice.jl
println("Twice of 2 is ", libwbnexus.nexus_start())
sleep(5)
