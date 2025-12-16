package player

import (
	"os"
)

func GenerateSpyScript() (string, error) {
	f, err := os.CreateTemp("", "mpv_spy_*.lua")
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Every 1 second, print "::STATUS::123.5/1400.0" to the console
	script := `
	mp.add_periodic_timer(1, function()
		local pos = mp.get_property("time-pos")
		local dur = mp.get_property("duration")
		if pos ~= nil and dur ~= nil then
			print("::STATUS::" .. tostring(pos) .. "/" .. tostring(dur))
		end
	end)

	mp.observe_property("sub-delay", "number", function(name, value)
		if value then
		    -- Format: ::SUB_DELAY::0.100000
		    print(string.format("::SUB_DELAY::%f", value))
		end
	    end)
	`

	f.WriteString(script)
	return f.Name(), nil
}
