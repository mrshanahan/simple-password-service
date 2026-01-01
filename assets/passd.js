var API_URL = "./api";
// var API_URL = "http://localhost:5555/admin/api";

function getCookie(name) {
    const cookies = document.cookie.split(';');
    const matchingCookies = cookies.filter(
        (v) => v.trim().split('=')[0] === name
    );
    if (matchingCookies.length === 0) {
        return null;
    }

    return matchingCookies[0].trim().split('=')[1];
}

function getUrlQueryParameter(name) {
    var queryString = window.location.search.substring(1);
    var queryArgs = queryString.split("&");
    var value = null;
    for (var i = 0; i < queryArgs.length; i++) {
        var pair = queryArgs[i].split("=");
        var key = decodeURIComponent(pair[0]);
        var v = decodeURIComponent(pair[1]);
        if (key === name) {
            value = v;
        }
    }

    return value;
}

function validateAuthToken() {
    const token = getCookie('access_token')
    if (!token) {
        window.location.href = "/auth/login?reason=unauthenticated"
    }
    return token
}

// TODO: This needs to be replaced by something that actually understands
//       HTML/markdown
function genericTextToHtmlText(text) {
    var lines = text.split(/\r?\n/);
    for (var i = 0; i < lines.length; i++) {
        var line = lines[i];
        line = line.replace('<', '&lt;');
        line = line.replace('>', '&gt;');
        while (line.match(/^\s+/)) {
            line = line.replace(/^(\s*)\s/, '$1&nbsp;');
        }
        while (line.match(/\s\s+/)) {
            line = line.replace(/(\s*)\s/, '$1&nbsp;');
        }
        lines[i] = line;
    }
    return lines.join('<br/>');
}

// API service interactions

function getEntries(token, callback, preReauthCallback) {
    genericSend('GET', API_URL + '/', token, callback, preReauthCallback, JSON.parse);
}

function getPassword(id, token, callback, preReauthCallback) {
    genericSend('GET', API_URL + '/' + id, token, callback, preReauthCallback, JSON.parse);
}

function upsertEntry(id, password, token, callback, preReauthCallback) {
    const json = JSON.stringify({ password: password });
    genericSendWithPayload('POST', API_URL + '/' + id, json, token, callback, preReauthCallback, 'application/json');
}

function deleteEntry(id, token, callback, preReauthCallback) {
    genericSend('DELETE', API_URL + '/' + id, token, callback, preReauthCallback);
}

function genericSend(method, url, token, callback, preReauthCallback, responseTransform) {
    const xhr = new XMLHttpRequest();
    xhr.open(method, url);
    xhr.setRequestHeader('Authorization', 'Bearer ' + token);
    xhr.onload = function() {
        if (xhr.status >= 200 && xhr.status < 300) {
            var response = xhr.response;
            if (responseTransform) {
                response = responseTransform(response);
            }
            console.log('Data received:', response);
            callback(response);
        } else if (xhr.status == 401) {
            console.error('Unauthenticated; redirecting');
            if (preReauthCallback) {
                preReauthCallback();
            }
            const origin = window.location.href;
            window.location.href = "/auth/login?origin_url=" + encodeURI(origin);
        } else {
            console.error('Request failed with status:', xhr.status);
        }
    };
    xhr.onerror = function() {
        console.error('Network error');
    };
    xhr.send();
}

function genericSendWithPayload(method, url, payload, token, callback, preReauthCallback, contentType = null, responseTransform) {
    const xhr = new XMLHttpRequest();
    xhr.open(method, url);
    xhr.setRequestHeader('Authorization', 'Bearer ' + token);
    if (contentType) {
        xhr.setRequestHeader('Content-Type', contentType);
    }
    xhr.onload = function() {
        if (xhr.status >= 200 && xhr.status < 300) {
            var response = xhr.response;
            if (responseTransform) {
                response = responseTransform(response);
            }
            console.log('Data received:', response);
            callback(response);
        } else if (xhr.status == 401) {
            console.error('Unauthenticated; redirecting');
            if (preReauthCallback) {
                preReauthCallback();
            }
            const origin = window.location.href;
            window.location.href = "/auth/login?origin_url=" + encodeURI(origin);
        } else {
            console.error('Request failed with status:', xhr.status);
        }
    };
    xhr.onerror = function() {
        console.error('Network error');
    };
    xhr.send(payload);
}