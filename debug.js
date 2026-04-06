const path = require('path');
const asar_dist_electron = 'D:\\\\code\\\\spider\\\\netmonitor\\\\release\\\\win-unpacked\\\\resources\\\\app.asar\\\\dist\\\\electron';
const combined = path.join(asar_dist_electron, '../frontend/index.html');
console.log('Path in asar context:', combined);
