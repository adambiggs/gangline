package main

func pending(name string) error {
	return refuseError("%s is unavailable until the Gangline 1.0 runtime is complete", name)
}

func (cmd command) up([]string) error        { return pending("up") }
func (cmd command) hitch([]string) error     { return pending("hitch") }
func (cmd command) adopt([]string) error     { return pending("adopt") }
func (cmd command) rename([]string) error    { return pending("rename") }
func (cmd command) send([]string) error      { return pending("send") }
func (cmd command) hostRun([]string) error   { return pending("run") }
func (cmd command) flush([]string) error     { return pending("flush") }
func (cmd command) queue([]string) error     { return pending("queue") }
func (cmd command) interrupt([]string) error { return pending("interrupt") }
func (cmd command) compact([]string) error   { return pending("compact") }
func (cmd command) context([]string) error   { return pending("context") }
func (cmd command) log([]string) error       { return pending("log") }
func (cmd command) replay([]string) error    { return pending("replay") }
func (cmd command) usage([]string) error     { return pending("usage") }
func (cmd command) cap([]string) error       { return pending("cap") }
func (cmd command) limits([]string) error    { return pending("limits") }
func (cmd command) wait([]string) error      { return pending("wait") }
func (cmd command) curfew([]string) error    { return pending("curfew") }
func (cmd command) status([]string) error    { return pending("status") }
func (cmd command) tick([]string) error      { return pending("tick") }
func (cmd command) hook([]string) error      { return pending("hook") }
func (cmd command) whoami([]string) error    { return pending("whoami") }
func (cmd command) roster([]string) error    { return pending("roster") }
func (cmd command) drop([]string) error      { return pending("drop") }
func (cmd command) down([]string) error      { return pending("down") }
func (cmd command) models([]string) error    { return pending("models") }
