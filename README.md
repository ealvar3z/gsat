# gsat (go-sway-alt-tab)

Golang daemon that alt-tabs and super-tabs windos and workspaces. It receives on SIGUSR{1,2}. Bindings bind to the same key in either direction (i.e. no need to Alt-Shift-Tab)

To change the window key combo from "Mod1+Tab", use the `-c` flag and set your preferred mapping.
To change the workspace key combo from "Mod4+Tab", use the `-w` flag and set your preferred mapping.


## Installation

Standard `go install`.

## License

gsat is licensed under the [MIT License](https://choosealicense.com/licenses/mit/).
