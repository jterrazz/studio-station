package machines

import "github.com/jterrazz/studio-station/src/page"

const thisMacWord = "This Mac"

func UseChosen(machine string, here page.Context) page.Page {
	return useSaid(here, "On "+machine+" in this terminal. Other terminals stay on This Mac.", page.Hint{Command: "station use -", Text: "to go back"})
}

func UseBack(to, was string, here page.Context) page.Page {
	return useSaid(here, "Back on "+usedName(to)+" (was "+usedName(was)+").")
}

func UseCurrent(machine, since string, here page.Context) page.Page {
	if machine == "" {
		return useSaid(here, "On This Mac in this terminal.", page.Hint{Command: "station use <machine>", Text: "to use another machine here"})
	}
	return useSaid(here, "On "+machine+" in this terminal since "+since, page.Hint{Command: "station use -", Text: "to go back"})
}

func usedName(machine string) string {
	if machine == "" {
		return thisMacWord
	}
	return machine
}

func useSaid(here page.Context, text string, hints ...page.Hint) page.Page {
	s := page.Section{ID: "use", Role: page.Banner, Uncounted: true, Items: []page.Item{{Note: &page.Note{Text: text}}}}
	return page.Seal(page.Page{ID: "use", Command: "use", Context: here, Sections: []page.Section{s}, Hints: hints})
}
