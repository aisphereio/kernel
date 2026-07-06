package project

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"

	"github.com/aisphereio/kernel/cmd/kernel/internal/base"
)

// Project is a project template.
type Project struct {
	Name string
	Path string
}

// New creates a project from a local layout or remote repo.
func (p *Project) New(ctx context.Context, dir string, layout string, branch string, opts ScaffoldOptions) error {
	to := filepath.Join(dir, p.Name)
	if _, err := os.Stat(to); !os.IsNotExist(err) {
		fmt.Printf("%s already exists\n", p.Name)
		prompt := &survey.Confirm{
			Message: "Do you want to override the folder ?",
			Help:    "Delete the existing folder and create the project.",
		}
		var override bool
		e := survey.AskOne(prompt, &override)
		if e != nil {
			return e
		}
		if !override {
			return err
		}
		os.RemoveAll(to)
	}
	fmt.Printf("Creating service %s, layout is %s.\n\n", p.Name, layout)
	repo := base.NewRepo(layout, branch)
	if err := repo.CopyTo(ctx, to, p.Name, []string{".git", ".github"}); err != nil {
		return err
	}
	if err := applyScaffoldOptions(to, opts); err != nil {
		return err
	}
	base.Tree(to, dir)

	fmt.Printf("\nProject creation succeeded %s\n", color.GreenString(p.Name))
	fmt.Print("Use the following commands to verify and run the service:\n\n")

	fmt.Println(color.WhiteString("$ cd %s", p.Name))
	fmt.Println(color.WhiteString("$ make tools"))
	fmt.Println(color.WhiteString("$ make api"))
	fmt.Println(color.WhiteString("$ make deploy"))
	fmt.Println(color.WhiteString("$ make proto-check"))
	fmt.Println(color.WhiteString("$ make verify"))
	fmt.Println(color.WhiteString("$ make run"))
	fmt.Println("\nThanks for using Aisphere Kernel")
	fmt.Println("Docs: https://github.com/aisphereio/kernel/tree/master/docs")
	return nil
}
