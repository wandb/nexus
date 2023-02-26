module wandb
            
# import .libwbnexus

# We write our Julia idiomatic function
function init()
    x = ccall((:nexus_start, "./libwbnexus"), Int32, ())
    j1 = ccall((:nexus_recv, "./libwbnexus"), Int32, (Int32,), x)
    ccall((:nexus_run_start, "./libwbnexus"), Cvoid, (Int32,), x)
    j1 = ccall((:nexus_recv, "./libwbnexus"), Int32, (Int32,), x)
    return x
end

function finish(i::Integer)
    ccall((:nexus_finish, "./libwbnexus"), Cvoid, (Int32,), i)
    j1 = ccall((:nexus_recv, "./libwbnexus"), Int32, (Int32,), i)
end

function log_scaler(i::Integer, k::String, v::Float32)
    ccall((:nexus_log_scaler, "./libwbnexus"), Cvoid, (Int32, Cstring, Float32), i, k, v)
end

end
