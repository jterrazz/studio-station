package kit

import "charm.land/bubbles/v2/key"

func Bind(help, desc string, keys ...string) key.Binding {
	return key.NewBinding(key.WithKeys(keys...), key.WithHelp(help, desc))
}

func MoveKey() key.Binding   { return Bind("↑↓", "move", "up", "down", "k", "j") }
func FoldKey() key.Binding   { return Bind("space", "fold", "space") }
func FilterKey() key.Binding { return Bind("/", "filter", "/") }
func DetailKey() key.Binding { return Bind("enter", "detail", "enter") }
