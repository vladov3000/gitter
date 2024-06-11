"use strict";

function onDocumentLoad() {
    const dates = document.getElementsByClassName("date");
    for (const date of dates) {
	const unixTimestamp = parseInt(date.textContent);
	date.textContent    = new Date(unixTimestamp * 1000).toLocaleDateString();
    }
}

document.addEventListener("DOMContentLoaded", onDocumentLoad);
