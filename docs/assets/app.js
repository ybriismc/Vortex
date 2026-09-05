// Documentation behaviour: highlights the section on screen in the sidebar,
// adds a copy button to every code block and drives the mobile navigation.
(function () {
  "use strict";

  // -- copy buttons ---------------------------------------------------------

  document.querySelectorAll("pre").forEach(function (pre) {
    const wrap = document.createElement("div");
    wrap.className = "pre-wrap";
    pre.parentNode.insertBefore(wrap, pre);
    wrap.appendChild(pre);

    const button = document.createElement("button");
    button.type = "button";
    button.className = "copy";
    button.textContent = "Copy";
    button.setAttribute("aria-label", "Copy the code to the clipboard");
    wrap.appendChild(button);

    let timer = null;
    button.addEventListener("click", function () {
      const done = function (label) {
        button.textContent = label;
        button.classList.toggle("copied", label === "Copied");
        clearTimeout(timer);
        timer = setTimeout(function () {
          button.textContent = "Copy";
          button.classList.remove("copied");
        }, 1600);
      };

      const text = pre.innerText;
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(
          function () { done("Copied"); },
          function () { done("Failed"); }
        );
        return;
      }

      // Fallback for browsers without the clipboard API.
      const area = document.createElement("textarea");
      area.value = text;
      area.setAttribute("readonly", "");
      area.style.position = "fixed";
      area.style.opacity = "0";
      document.body.appendChild(area);
      area.select();
      try {
        done(document.execCommand("copy") ? "Copied" : "Failed");
      } catch (err) {
        done("Failed");
      }
      document.body.removeChild(area);
    });
  });

  // -- sidebar highlighting -------------------------------------------------

  const links = Array.from(document.querySelectorAll(".nav a"));
  const sections = links
    .map(function (link) {
      const href = link.getAttribute("href") || "";
      return href.startsWith("#") ? document.querySelector(href) : null;
    })
    .filter(Boolean);

  if ("IntersectionObserver" in window && sections.length) {
    const observer = new IntersectionObserver(
      function (entries) {
        entries.forEach(function (entry) {
          if (!entry.isIntersecting) {
            return;
          }

          links.forEach(function (link) {
            link.classList.toggle("active", link.getAttribute("href") === "#" + entry.target.id);
          });
        });
      },
      { rootMargin: "-10% 0px -75% 0px", threshold: 0 }
    );
    sections.forEach(function (section) { observer.observe(section); });
  }

  // -- mobile navigation ----------------------------------------------------

  const sidebar = document.querySelector(".sidebar");
  const toggle = document.querySelector(".menu-toggle");
  if (!sidebar || !toggle) {
    return;
  }

  const scrim = document.createElement("div");
  scrim.className = "scrim";
  document.body.appendChild(scrim);

  const setOpen = function (open) {
    sidebar.classList.toggle("open", open);
    document.body.classList.toggle("nav-open", open);
    toggle.textContent = open ? "Close" : "Menu";
    toggle.setAttribute("aria-expanded", String(open));
  };

  toggle.setAttribute("aria-expanded", "false");
  toggle.addEventListener("click", function () { setOpen(!sidebar.classList.contains("open")); });
  scrim.addEventListener("click", function () { setOpen(false); });

  sidebar.addEventListener("click", function (event) {
    if (event.target.tagName === "A") {
      setOpen(false);
    }
  });

  document.addEventListener("keydown", function (event) {
    if (event.key === "Escape") {
      setOpen(false);
    }
  });
})();
