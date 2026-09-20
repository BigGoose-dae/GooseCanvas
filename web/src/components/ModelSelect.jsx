import { useEffect, useId, useRef, useState } from "react";

export default function ModelSelect({ label, options, value, onChange }) {
  const [open, setOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState(0);
  const rootRef = useRef(null);
  const listboxId = useId();
  const selectedIndex = Math.max(
    0,
    options.findIndex((option) => option.key === value),
  );
  const selected = options[selectedIndex];

  useEffect(() => {
    if (!open) return undefined;
    const close = (event) => {
      if (!rootRef.current?.contains(event.target)) setOpen(false);
    };
    window.addEventListener("pointerdown", close);
    return () => window.removeEventListener("pointerdown", close);
  }, [open]);

  const show = () => {
    setActiveIndex(selectedIndex);
    setOpen(true);
  };

  const choose = (index) => {
    const option = options[index];
    if (!option) return;
    if (option.key !== value) onChange(option.key);
    setOpen(false);
  };

  const onKeyDown = (event) => {
    if (!options.length) return;
    if (!open) {
      if (["ArrowDown", "ArrowUp", "Enter", " "].includes(event.key)) {
        event.preventDefault();
        show();
      }
      return;
    }
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setActiveIndex((index) => (index + 1) % options.length);
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      setActiveIndex((index) => (index - 1 + options.length) % options.length);
    } else if (event.key === "Home") {
      event.preventDefault();
      setActiveIndex(0);
    } else if (event.key === "End") {
      event.preventDefault();
      setActiveIndex(options.length - 1);
    } else if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      choose(activeIndex);
    } else if (event.key === "Escape") {
      event.preventDefault();
      setOpen(false);
    } else if (event.key === "Tab") {
      setOpen(false);
    }
  };

  return (
    <div className="model-select" ref={rootRef}>
      <button
        type="button"
        className="model-select-trigger"
        aria-label={label}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={open ? listboxId : undefined}
        disabled={!options.length}
        onClick={() => (open ? setOpen(false) : show())}
        onKeyDown={onKeyDown}
      >
        <span className="model-select-value">
          <strong>{selected?.name || "—"}</strong>
          {selected?.key && <small>{selected.key}</small>}
        </span>
        <span className="model-select-chevron" aria-hidden="true">⌄</span>
      </button>
      {open && (
        <div
          id={listboxId}
          className="model-select-menu"
          role="listbox"
          aria-label={label}
          aria-activedescendant={`${listboxId}-${activeIndex}`}
        >
          {options.map((option, index) => (
            <button
              id={`${listboxId}-${index}`}
              type="button"
              role="option"
              aria-selected={option.key === selected?.key}
              className={`model-select-option ${index === activeIndex ? "active" : ""}`}
              key={option.key}
              onMouseMove={() => setActiveIndex(index)}
              onClick={() => choose(index)}
            >
              <span>
                <strong>{option.name}</strong>
                <small>{option.key}</small>
              </span>
              {option.key === selected?.key && <b aria-hidden="true">✓</b>}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
