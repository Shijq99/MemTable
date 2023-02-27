---
--- Generated by EmmyLua(https://github.com/EmmyLua)
--- Created by tangrenchu.
--- DateTime: 2023/2/12 00:43
--- 由于 Gopher-Lua 中不支持 Lua 的调试模式，因此这里使用判别名称这一简单的方式来禁用全局变量

setmetatable(_G, {
            __newindex = function (t, n, v)
               if string.sub(n,1,2) ~= 'f_' then
               	    error("Script attempted to create global variable '\"..tostring(n)..\"'", 2)
               else
                    rawset(t, n, v)
               end
            end,

            __index = function (t, n, v)
               if string.sub(n,1,2) ~= 'f_' then
                    error("attempt to read undeclared variable '"..tostring(n).."'", 2)
               end
            end,
        })