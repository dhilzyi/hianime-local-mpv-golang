local tracksub = nil
local trackpos = nil

mp.add_periodic_timer(1, function()
	local pos = mp.get_property("time-pos")
	local dur = mp.get_property("duration")
	if pos ~= nil and dur ~= nil then
		if pos ~= trackpos then
			trackpos = pos
			-- Format: ::STATUS::10/1420.0000
			print("::STATUS::" .. tostring(pos) .. "/" .. tostring(dur))
		end
	end

	mp.observe_property("sub-delay", "number", function(name, value)
		if value and value ~= tracksub then
			tracksub = value

			-- Format: ::SUB_DELAY::0.100000
			print(string.format("::SUB_DELAY::%f", value))
		end
	end)
end)
