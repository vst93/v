package plugin_template

type Template struct {
	name        string
	version     string
	description string
	command     string
	args        map[string]string
	author      string
}

func (t *Template) Init() error {
	t.name = "template"
	t.version = "0.0.1"
	t.description = "This is a template plugin."
	t.command = "template"
	t.args = map[string]string{
		"-v": "version of the plugin",
	}
	t.author = ""
	return nil
}

func (t *Template) GetName() string {
	return t.name
}
func (t *Template) GetVersion() string {
	return t.version
}
func (t *Template) GetDescription() string {
	return t.description
}
func (t *Template) GetCommand() string {
	return t.command
}
func (t *Template) GetArgs() map[string]string {
	return t.args
}
func (t *Template) GetAuthor() string {
	return t.author
}

func (t *Template) Run(args []string) error {
	// TODO: implement your plugin logic here
	return nil
}
func (t *Template) Stop() error {
	return nil
}
