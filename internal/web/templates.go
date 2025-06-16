// Lab 7, 8, 9: Use these templates to render the web pages
package web

const indexHTML = `
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <title>TritonTube</title>

    <!-- JetBrains Mono font import -->
    <link href="https://fonts.googleapis.com/css2?family=JetBrains+Mono&display=swap" rel="stylesheet" />

    <style>
      :root {
        --primary: #3498db;
        --primary-dark: #2980b9;
        --background: #1e1e1e;
        --card-bg: #2c2c2c;
        --text: #ffffff;
        --accent: #2ecc71;
        --accent-dark: #27ae60;
        --border-color: rgba(255, 255, 255, 0.1);
        --code-font: 'JetBrains Mono', 'Fira Code', 'Courier New', monospace;
      }

      body {
        background: var(--background);
        color: var(--text);
        font-family: var(--code-font);
        margin: 0;
        padding: 40px 20px;
        max-width: 800px;
        margin-inline: auto;
      }

      h1 {
        font-size: 2.8em;
        margin-bottom: 10px;
        color: var(--primary);
        text-align: center;
      }

      h2 {
        font-size: 1.5em;
        margin: 40px 0 15px;
        color: var(--primary-dark);
      }

      form {
        background: var(--card-bg);
        padding: 20px;
        border-radius: 12px;
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
        display: flex;
        flex-wrap: wrap;
        gap: 12px;
        border: 1px solid var(--border-color);
      }

      input[type="file"] {
        flex: 1 1 260px;
        padding: 0;
        background: transparent;
        color: var(--text);
        border: none;
        cursor: pointer;
        font-family: var(--code-font);
      }

      input[type="file"]::-webkit-file-upload-button {
        background: var(--primary);
        color: #fff;
        border: none;
        padding: 10px 18px;
        border-radius: 999px;
        font-weight: 600;
        cursor: pointer;
        transition: background 0.3s;
        font-family: var(--code-font);
      }

      input[type="file"]::-webkit-file-upload-button:hover {
        background: var(--primary-dark);
      }

      input[type="file"]::file-upload-button {
        background: var(--primary);
        color: #fff;
        border: none;
        padding: 10px 18px;
        border-radius: 999px;
        font-weight: 600;
        cursor: pointer;
        transition: background 0.3s;
        font-family: var(--code-font);
      }

      input[type="file"]::file-upload-button:hover {
        background: var(--primary-dark);
      }

      input[type="submit"] {
        background: var(--accent);
        color: #fff;
        border: none;
        padding: 10px 22px;
        border-radius: 999px;
        font-weight: 600;
        cursor: pointer;
        transition: background 0.3s;
        font-family: var(--code-font);
      }

      input[type="submit"]:hover {
        background: var(--accent-dark);
      }

      ul {
        list-style: none;
        padding: 0;
        margin-top: 10px;
      }

      li {
        background: var(--card-bg);
        padding: 12px 16px;
        margin-bottom: 10px;
        border-radius: 8px;
        transition: transform 0.25s ease, box-shadow 0.25s ease;
        box-shadow: 0 2px 8px rgba(0, 0, 0, 0.25);
      }

      li:hover {
        transform: translateY(-4px) scale(1.03);
        box-shadow: 0 6px 14px rgba(0, 0, 0, 0.4);
      }

      a {
        color: var(--accent);
        text-decoration: none;
        font-weight: 600;
        font-family: var(--code-font);
      }

      a:hover {
        color: var(--accent-dark);
      }
    </style>
  </head>
  <body>
    <h1>Welcome to TritonTube</h1>

    <h2>Upload an MP4 Video</h2>
    <form action="/upload" method="post" enctype="multipart/form-data">
      <input type="file" name="file" accept="video/mp4" required />
      <input type="submit" value="Upload" />
    </form>

    <h2>Watchlist</h2>
    <ul>
      {{range .}}
      <li>
        <a href="/videos/{{.EscapedID}}">{{.Id}} ({{.UploadTime}})</a>
      </li>
      {{else}}
      <li>No videos uploaded yet.</li>
      {{end}}
    </ul>
  </body>
</html>
`

const videoHTML = `
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <title>{{.Id}} - TritonTube</title>

    <!-- JetBrains Mono font -->
    <link href="https://fonts.googleapis.com/css2?family=JetBrains+Mono&display=swap" rel="stylesheet" />
    <script src="https://cdn.dashjs.org/latest/dash.all.min.js"></script>

    <style>
      :root {
        --primary: #3498db;
        --primary-dark: #2980b9;
        --background: #1e1e1e;
        --card-bg: #2c2c2c;
        --text: #ffffff;
        --accent: #2ecc71;
        --accent-dark: #27ae60;
        --border-color: rgba(255, 255, 255, 0.1);
        --code-font: 'JetBrains Mono', 'Fira Code', 'Courier New', monospace;
      }

      body {
        background: var(--background);
        color: var(--text);
        font-family: var(--code-font);
        margin: 0;
        padding: 40px 20px;
        max-width: 800px;
        margin-inline: auto;
      }

      h1 {
        font-size: 2.4em;
        margin-bottom: 10px;
        color: var(--primary);
      }

      p {
        font-size: 1rem;
        margin: 8px 0 20px;
        color: #ccc;
      }

      video {
        width: 100%;
        max-width: 100%;
        border-radius: 12px;
        background-color: black;
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.4);
        margin-bottom: 24px;
      }

      a {
        display: inline-block;
        padding: 10px 20px;
        background-color: var(--accent);
        color: white;
        text-decoration: none;
        font-weight: bold;
        border-radius: 999px;
        transition: background-color 0.3s ease;
      }

      a:hover {
        background-color: var(--accent-dark);
      }
    </style>
  </head>
  <body>
    <h1>{{.Id}}</h1>
    <p>Uploaded at: {{.UploadedAt}}</p>

    <video id="dashPlayer" controls></video>

    <script>
      var url = "/content/{{.Id}}/manifest.mpd";
      var player = dashjs.MediaPlayer().create();
      player.initialize(document.querySelector("#dashPlayer"), url, false);
    </script>

    <p><a href="/">← Back to Home</a></p>
  </body>
</html>
`
