module ramserver-tictactoe

go 1.24

require github.com/stevenlagoy/ramserver-core/server v0.0.0

// The server's public game package is provided in ./sdk. Never fetch the real module.
replace github.com/stevenlagoy/ramserver-core/server => ./sdk
