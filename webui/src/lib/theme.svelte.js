// Warm is a stored choice, never a probe: the OS preference says nothing about whether
// this page should dim, and both modes are light-grounded anyway.
const stored = "p9-theme";
const warmName = "warm";

export const warm = $state({ on: false });

export function loadTheme() {
	apply(localStorage.getItem(stored) === warmName);
}

export function toggleTheme() {
	apply(!warm.on);
	localStorage.setItem(stored, warm.on ? warmName : "");
}

function apply(on) {
	warm.on = on;
	if (on) {
		document.documentElement.dataset.theme = warmName;
	} else {
		delete document.documentElement.dataset.theme;
	}
}
