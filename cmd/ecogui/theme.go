package main

import (
	"image/color"

	"fyne.io/fyne"
	"fyne.io/fyne/theme"
)

var (
	defaultBorderColor = stringToColor("#333")
)

type defaultTheme struct {
	fyne.Theme
	regularFont *fyne.StaticResource
	boldFont    *fyne.StaticResource
}

func newDefaultTheme() *defaultTheme {
	return &defaultTheme{
		Theme:       theme.DarkTheme(),
		regularFont: fontRegular,
		boldFont:    fontBold,
	}
}

// 2.0.0 interface

// func (t *defaultTheme) Color(cName fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
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

// func (t *defaultTheme) Size(sName fyne.ThemeSizeName) int {
// 	return t.Theme.Size(sName)
// }

// func (t *defaultTheme) Font(style fyne.TextStyle) fyne.Resource {
// 	return t.Theme.Font(style)
// }

// Pre 2.0.0 interface

func (t *defaultTheme) BackgroundColor() color.Color {
	return bgColor
}

func (t *defaultTheme) Padding() int {
	return 0
}

func (t *defaultTheme) ButtonColor() color.Color {
	return bttnColor
}

func (t *defaultTheme) TextFont() fyne.Resource {
	return t.regularFont
}

func (t *defaultTheme) TextBoldFont() fyne.Resource {
	return t.boldFont
}

func (t *defaultTheme) TextColor() color.Color {
	return textColor
}

func (t *defaultTheme) TextSize() int {
	return 15
}

// func (t *defaultTheme) ShadowColor() color.Color {
// 	return transparent
// }

func (t *defaultTheme) FocusColor() color.Color {
	return focusColor
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
