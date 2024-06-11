"use strict";

const parameters = new URLSearchParams(window.location.search);
const pageParameter = parameters.get("page")

let page = 0;
if (pageParameter !== null)  {
    const parsedPage = parseInt(pageParameter);
    if (parsedPage !== NaN) {
	page = parsedPage;
    }
}

function onPrevious() {
    window.location.assign(`/?page=${page - 1}`);
}

function onNext() {
    window.location.assign(`/?page=${page + 1}`);
}

function onDocumentLoad() {
    const dates = document.getElementsByClassName("date");
    for (const date of dates) {
	const unixTimestamp = parseInt(date.textContent);
	date.textContent    = new Date(unixTimestamp * 1000).toLocaleDateString();
    }

    const previous = document.getElementById("previous");
    if (page === 0) {
	previous.style.visibility = "hidden";
    } else {
	previous.addEventListener("click", onPrevious);
    }

    const next = document.getElementById("next");
    next.addEventListener("click", onNext);
}

document.addEventListener("DOMContentLoaded", onDocumentLoad);
