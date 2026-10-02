package niri

import "testing"

func TestViewFollowsEvents(t *testing.T) {
	s := NewState()
	lines := []string{
		`{"Ok":"Handled"}`,
		`{"WorkspacesChanged":{"workspaces":[{"id":1,"idx":1,"is_focused":true},{"id":2,"idx":2,"is_focused":false}]}}`,
		`{"WindowsChanged":{"windows":[
			{"id":10,"title":"fish","app_id":"org.wezfurlong.wezterm","workspace_id":1,"is_focused":false,"is_floating":false,
			 "layout":{"pos_in_scrolling_layout":[1,1],"tile_size":[640,800],"tile_pos_in_workspace_view":[-632,0]}},
			{"id":11,"title":"docs","app_id":"helium","workspace_id":1,"is_focused":true,"is_floating":false,
			 "layout":{"pos_in_scrolling_layout":[2,1],"tile_size":[960,800],"tile_pos_in_workspace_view":[16,0]}},
			{"id":12,"title":"calc","app_id":"org.gnome.Calculator","workspace_id":1,"is_focused":false,"is_floating":true,
			 "layout":{"pos_in_scrolling_layout":null,"tile_size":[300,400],"tile_pos_in_workspace_view":null}},
			{"id":13,"title":"other","app_id":"x","workspace_id":2,"is_focused":false,"is_floating":false,
			 "layout":{"pos_in_scrolling_layout":[1,1],"tile_size":[500,800],"tile_pos_in_workspace_view":null}}
		]}}`,
		`{"WindowOpenedOrChanged":{"window":{"id":14,"title":"notes","app_id":"org.gnome.TextEditor","workspace_id":1,"is_focused":true,"is_floating":false,
			"layout":{"pos_in_scrolling_layout":[2,2],"tile_size":[900,400],"tile_pos_in_workspace_view":[16,400]}}}}`,
		`{"WindowLayoutsChanged":{"changes":[[10,{"pos_in_scrolling_layout":[1,1],"tile_size":[700,800],"tile_pos_in_workspace_view":[-692,0]}]]}}`,
	}
	for _, l := range lines {
		if err := s.Apply([]byte(l)); err != nil {
			t.Fatalf("apply %s: %v", l, err)
		}
	}

	v := s.View(8)
	if v.Focused == nil || v.Focused.ID != 14 {
		t.Fatalf("focused = %+v, want window 14", v.Focused)
	}
	if s.Windows[11].IsFocused {
		t.Fatal("window 11 still focused after 14 opened focused")
	}
	if len(v.Columns) != 2 {
		t.Fatalf("columns = %+v, want 2 (floating and other-workspace windows excluded)", v.Columns)
	}
	if v.Columns[0].Width != 700 || v.Columns[1].X != 708 || v.Columns[1].Width != 960 || !v.Columns[1].Focused {
		t.Fatalf("columns = %+v", v.Columns)
	}
	if v.ViewX != 692 {
		t.Fatalf("ViewX = %v, want 692", v.ViewX)
	}

	if err := s.Apply([]byte(`{"WindowClosed":{"id":14}}`)); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply([]byte(`{"WindowFocusChanged":{"id":null}}`)); err != nil {
		t.Fatal(err)
	}
	if v := s.View(8); v.Focused != nil || len(v.Columns) != 2 {
		t.Fatalf("after close: focused=%+v columns=%d", v.Focused, len(v.Columns))
	}
}
