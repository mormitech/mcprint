import './style.css';
import {Version} from '../wailsjs/go/main/App';

async function init() {
  document.getElementById('app').innerHTML =
    `<h1>mcprint</h1><p>Scan pages of documents, on any platform.</p><p>Skeleton: no scanner support yet.</p><p class="ver">mcprint v${await Version()}</p>`;
}

init();
