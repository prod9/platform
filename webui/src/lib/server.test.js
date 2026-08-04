import { describe, expect, test } from "vitest";
import {
	errorText,
	installSignal,
	Answered,
	Refused,
	Offline,
	Installing,
	Installed,
	Unknown,
} from "./server.js";

describe("errorText", () => {
	test("gives Offline a human-readable message", () => {
		expect(errorText({ outcome: Offline, body: "" })).toBe(
			"No answer from the platform server.",
		);
	});

	test("a refusal shows the handler's reason", () => {
		expect(errorText({ outcome: Refused, body: "ref not found", status: 422 })).toBe(
			"ref not found",
		);
	});

	test("a reasonless refusal still says something", () => {
		expect(errorText({ outcome: Refused, body: "", status: 502 })).toBe(
			"The server refused without a reason (status 502).",
		);
	});
});

describe("installSignal", () => {
	test("an answered checklist means the installer is up", () => {
		expect(installSignal({ outcome: Answered, body: [] })).toBe(Installing);
	});

	test("only a real 404 means installed", () => {
		expect(installSignal({ outcome: Refused, body: "", status: 404 })).toBe(Installed);
	});

	test("a server error is not the installed signal", () => {
		expect(installSignal({ outcome: Refused, body: "boom", status: 500 })).toBe(Unknown);
	});

	test("no answer at all is neither side", () => {
		expect(installSignal({ outcome: Offline, body: "" })).toBe(Unknown);
	});
});
