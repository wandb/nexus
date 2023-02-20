module libwbnexus
            
# We write our Julia idiomatic function
function nexus_start()
    ccall((:nexus_start, "./libwbnexus"), Int32, ())
end

end
