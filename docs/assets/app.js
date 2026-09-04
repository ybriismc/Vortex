// Highlights the section currently on screen in the sidebar and drives the
// mobile navigation toggle.
(function () {
  const links = Array.from(document.querySelectorAll(".nav a"));
  const sections = links
    .map((link) => document.querySelector(link.getAttribute("href")))
    .filter(Boolean);

  if ("IntersectionObserver" in window && sections.length) {
    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (!entry.isIntersecting) {
            return;
          }

          links.forEach((link) => {
            link.classList.toggle("active", link.getAttribute("href") === "#" + entry.target.id);
          });
        });
      },
      { rootMargin: "-10% 0px -75% 0px", threshold: 0 }
    );
    sections.forEach((section) => observer.observe(section));
  }

  const sidebar = document.querySelector(".sidebar");
  const toggle = document.querySelector(".menu-toggle");
  if (!sidebar || !toggle) {
    return;
  }

  toggle.addEventListener("click", () => {
    const open = sidebar.classList.toggle("open");
    toggle.textContent = open ? "Close" : "Menu";
  });

  sidebar.addEventListener("click", (event) => {
    if (event.target.tagName === "A") {
      sidebar.classList.remove("open");
      toggle.textContent = "Menu";
    }
  });
})();
