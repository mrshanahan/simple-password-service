var entries;
var token;
var hasNewEntry;

function el(elName, className, attrs, children) {
    const element = document.createElement(elName);
    element.className = className;
    for (var k in attrs) {
        if (k === 'onclick') {
            element.onclick = attrs[k];
        } else {
            element.setAttribute(k, attrs[k]);
        }
    }

    if (children) {
        for (var c of children) {
            if (typeof c !== 'object') {
                element.innerText = c;
            } else {
                element.appendChild(c);
            }
        }
    }
    return element;
}

function handleDelete(id) {
    // TODO: Handle error
    deleteEntry(id, token, () => {
        entries = entries.filter(e => e.id !== id);
        renderEntries(entries);
    })
}

function handleCreate(id, password) {
    upsertEntry(id, password, token, (resp) => {
        hasNewEntry = false;
        const entry = { id: id };
        entries = entries.concat([entry]);
        renderEntries(entries);
    })
}

function handleRevealPassword(id) {
    getPassword(id, token, (resp) => {
        const entryIdx = entries.findIndex(e => e.id == id);
        if (entryIdx >= 0) {
            const entry = entries[entryIdx];
            entries[entryIdx] = { ...entry, password: resp.password };
            renderEntries(entries);
        }
    })
}

function handleUpdatePassword(id, password) {
    upsertEntry(id, password, token, (resp) => {
        const entryIdx = entries.findIndex(e => e.id == id);
        if (entryIdx >= 0) {
            const entry = entries[entryIdx];
            entries[entryIdx] = { ...entry, password: password };
            renderEntries(entries);
        }
    })
}

function extractEntryId(elementId) {
    const base64Id = /entry-password-(.*)/.exec(elementId);
    return atob(base64Id);
}

function getPasswordInputElements(elementId, entry) {
    if (entry.password == null) {
        return [
            el('input', '', {id: elementId, type: 'text', disabled: true, placeholder: 'Click icon to reveal'}),
            el('button', 'entry-reveal-password-button', {onclick: () => handleRevealPassword(entry.id)}, [
                el('img', '', {src: './eye.svg', width: '20px', height: '20px'})
            ])
        ];
    }
    return [
        el('input', '', {id: elementId, type: 'text', value: entry.password}),
        el('button', 'entry-update-password-button', {onclick: () => {
            const passwordInput = document.getElementById(elementId);
            const password = passwordInput.value;
            handleUpdatePassword(entry.id, password);
        }}, [
            el('img', '', {src: './floppy-disk.svg', width: '20px', height: '20px'})
        ])
    ]
}

function constructEntry(entry) {
    console.log(entry);
    const id = entry.id;
    const password = entry.password;
    const base64Id = btoa(id);
    const entryElementId = `entry-${base64Id}`;
    const entryPasswordElementId = `entry-password-${base64Id}`;
    const entryNode =
        el('div', 'entry-container', {}, [
            el('div', 'entry', {id: entryElementId}, [
                el('div', 'entry-top-bar', {}, [
                    el('label', '', {for: entryPasswordElementId}, [id]),
                    el('button', 'entry-delete-button', {onclick: () => {
                        const confirmed = confirm(`Are you sure you want to delete entry ${id}?`)
                        if (confirmed) {
                            handleDelete(id);
                        }
                    }}, ['X'])
                ]),
                el('div', 'entry-bottom-bar', {}, getPasswordInputElements(entryPasswordElementId, entry))
            ])
        ]);

    return entryNode;
}

function constructAddEntryNode() {
    return el('div', 'entry-create-button-container', {}, [
        el('button', '', {id: 'entry-new-add-button', onclick: () => { hasNewEntry = true; renderEntries(entries); }}, [
            el('img', '', {src: './plus.svg', width: '40px', height: '40px'})
        ])
    ]);
}

function constructNewEntryNode() {
    return el('div', 'entry-container', {}, [
        el('div', 'entry', {id: `entry-new`}, [
            el('div', 'entry-top-bar', {}, [
                el('input', '', {id: 'entry-new-id', type: 'text', placeholder: 'Enter id here'}),
                el('button', 'entry-delete-button', {onclick: () => { hasNewEntry = false; renderEntries(entries); }}, ['X'])
            ]),
            el('div', 'entry-bottom-bar', {}, [
                el('input', '', {id: 'entry-new-password', type: 'password', placeholder: 'Enter password here'}),
                el('button', 'entry-new-create-button', {onclick: (e) => {
                    const id = document.getElementById('entry-new-id').value;
                    const password = document.getElementById('entry-new-password').value;
                    handleCreate(id, password);
                }}, [
                    el('img', '', {src: './floppy-disk.svg', width: '20px', height: '20px'})
                ])
            ])
        ])
    ]);
}

function renderEntries(es) {
    const container = document.getElementById("entries-container");
    container.replaceChildren([]);
    for (var e of es) {
        const entryNode = constructEntry(e);
        container.appendChild(entryNode);
    }
    if (hasNewEntry) {
        container.appendChild(constructNewEntryNode());
    } else {
        container.appendChild(constructAddEntryNode());
    }
}

function loadEntries() {
    token = validateAuthToken();
    if (!token) {
        return;
    }

    getEntries(token, function (es) {
        entries = es.map(e => { return { ...e, password: null }});
        renderEntries(entries);
    });
}

window.onload = () => {
    loadEntries();
}