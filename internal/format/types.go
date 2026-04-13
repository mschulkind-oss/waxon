// Package format implements the .slides file format parser and writer.
package format

// Deck represents a complete parsed .slides file.
type Deck struct {
	Meta   Meta    `json:"meta"`
	Slides []Slide `json:"slides"`
}

// Meta holds the YAML frontmatter metadata.
type Meta struct {
	Title           string `yaml:"title" json:"title"`
	Author          string `yaml:"author,omitempty" json:"author,omitempty"`
	Date            string `yaml:"date,omitempty" json:"date,omitempty"`
	Theme           string `yaml:"theme,omitempty" json:"theme,omitempty"`
	Aspect          string `yaml:"aspect,omitempty" json:"aspect,omitempty"`
	Footer          string `yaml:"footer,omitempty" json:"footer,omitempty"`
	Transition      string `yaml:"transition,omitempty" json:"transition,omitempty"`
	TerminalVariant string `yaml:"terminal-variant,omitempty" json:"terminal_variant,omitempty"`
	TerminalEffects bool   `yaml:"terminal-effects,omitempty" json:"terminal_effects,omitempty"`
}

// Slide represents a single slide in the deck.
type Slide struct {
	Index     int        `json:"index"`
	ID        string     `json:"id,omitempty"`
	Content   string     `json:"content"`
	Notes     []string   `json:"notes,omitempty"`
	AINotes   []string   `json:"ai_notes,omitempty"`
	Comments  []Comment  `json:"comments,omitempty"`
	Variants  []Variant  `json:"variants,omitempty"`
	Pauses    int        `json:"pauses,omitempty"`
	SlideOpts *SlideOpts `json:"slide_opts,omitempty"`
}

// Comment represents an inline review comment.
type Comment struct {
	Author string `json:"author"`
	Text   string `json:"text"`
}

// Variant represents an alternate version of a slide. Variants carry their
// own directives — notes, ai notes, comments, and pause counts — so a
// reviewer can leave feedback on a specific alternative without affecting
// the main slide or its siblings.
type Variant struct {
	Name     string    `json:"name"`
	Content  string    `json:"content"`
	Notes    []string  `json:"notes,omitempty"`
	AINotes  []string  `json:"ai_notes,omitempty"`
	Comments []Comment `json:"comments,omitempty"`
	Pauses   int       `json:"pauses,omitempty"`
}

// SlideOpts represents per-slide overrides from <!-- slide: ... --> directives.
type SlideOpts struct {
	Background string `json:"bg,omitempty"`
	Class      string `json:"class,omitempty"`
}
