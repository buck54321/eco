package ui

import (
	"image/color"

	"fyne.io/fyne"
	"fyne.io/fyne/theme"
)

var (
	DefaultBorderColor = StringToColor("#333")
	BttnColor          = StringToColor("#005")
	BgColor            = StringToColor("#000006")
	BlockerColor       = StringToColor("#000006aa")
	Transparent        = StringToColor("#0000")
	White              = StringToColor("#fff")

	DecredKeyBlue   = StringToColor("#2970ff")
	DecredTurquoise = StringToColor("#2ed6a1")
	DecredDarkBlue  = StringToColor("#091440")
	DecredLightBlue = StringToColor("#70cbff")
	DecredGreen     = StringToColor("#41bf53")
	DecredOrange    = StringToColor("#ed6d47")

	DefaultButtonColor      = StringToColor("#003")
	DefaultButtonHoverColor = StringToColor("#005")
	ButtonColor2            = StringToColor("#001a08")
	ButtonHoverColor2       = StringToColor("#00251a")
	TextColor               = StringToColor("#c1c1c1")
	CursorColor             = StringToColor("#2970fe")
	FocusColor              = CursorColor
	Black                   = StringToColor("#000")
	InputColor              = StringToColor("#111")

	RegularFont = &fyne.StaticResource{
		StaticName:    "source-sans.ttf",
		StaticContent: SourceSans,
	}

	BoldFont = &fyne.StaticResource{
		StaticName:    "source-sans-semibold.ttf",
		StaticContent: SourceSansSemibold,
	}
)

type DefaultTheme struct {
	fyne.Theme
}

func NewDefaultTheme() *DefaultTheme {
	return &DefaultTheme{
		Theme: theme.DarkTheme(),
	}
}

// 2.0.0 interface

// func (t *DefaultTheme) Color(cName fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
// 	switch cName {
// 	case theme.Colors.Background: // red,
// 		return bgColor
// 	// case theme.Colors.Button:          // color.Black,
// 	// case theme.Colors.Disabled:        // color.Black,
// 	// case theme.Colors.DisabledButton:  // color.White,
// 	case theme.Colors.Focus: // green,
// 		return focusColor
// 	case theme.Colors.Foreground: // color.White,
// 		return textColor
// 		// case theme.Colors.Hover:           // green,
// 		// case theme.Colors.PlaceHolder:     // blue,
// 		// case theme.Colors.Primary:         // green,
// 		// case theme.Colors.ScrollBar:       // blue,
// 		// case theme.Colors.Shadow:          // blue,
// 	}
// 	return t.Theme.Color(cName, variant)
// }

// func (t *DefaultTheme) Size(sName fyne.ThemeSizeName) int {
// 	return t.Theme.Size(sName)
// }

// func (t *DefaultTheme) Font(style fyne.TextStyle) fyne.Resource {
// 	return t.Theme.Font(style)
// }

// Pre 2.0.0 interface

func (t *DefaultTheme) BackgroundColor() color.Color {
	return BgColor
}

func (t *DefaultTheme) Padding() int {
	return 0
}

func (t *DefaultTheme) ButtonColor() color.Color {
	return BttnColor
}

func (t *DefaultTheme) TextFont() fyne.Resource {
	return RegularFont
}

func (t *DefaultTheme) TextBoldFont() fyne.Resource {
	return BoldFont
}

func (t *DefaultTheme) TextColor() color.Color {
	return TextColor
}

func (t *DefaultTheme) TextSize() int {
	return 15
}

// func (t *DefaultTheme) ShadowColor() color.Color {
// 	return transparent
// }

func (t *DefaultTheme) FocusColor() color.Color {
	return FocusColor
}

// DisabledButtonColor() color.Color
// // Deprecated: Hyperlinks now use the primary color for consistency.
// HyperlinkColor() color.Color
// DisabledTextColor() color.Color
// // Deprecated: Icons now use the text colour for consistency.
// IconColor() color.Color
// // Deprecated: Disabled icons match disabled text color for consistency.
// DisabledIconColor() color.Color
// PlaceHolderColor() color.Color
// PrimaryColor() color.Color
// HoverColor() color.Color
// FocusColor() color.Color
// ScrollBarColor() color.Color
// ShadowColor() color.Color

// TextSize() int
// TextFont() Resource

// TextItalicFont() Resource
// TextBoldItalicFont() Resource
// TextMonospaceFont() Resource

// IconInlineSize() int
// ScrollBarSize() int
// ScrollBarSmallSize() int
