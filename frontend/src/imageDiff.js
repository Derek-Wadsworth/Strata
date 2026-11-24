/** Image comparison helpers: analysis + mode renderers. */

export function isImagePath(path) {
  return /\.(png|jpe?g|gif|webp|bmp)$/i.test(String(path || ""));
}

export function formatBytes(n) {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

export function mediaURL(media) {
  if (!media?.base64) return "";
  return `data:${media.mime || "application/octet-stream"};base64,${media.base64}`;
}

export function loadHTMLImage(url) {
  return new Promise((resolve, reject) => {
    const img = new Image();
    img.onload = () => resolve(img);
    img.onerror = () => reject(new Error("failed to decode image"));
    img.src = url;
  });
}

function canvasFromImage(img, w, h) {
  const c = document.createElement("canvas");
  c.width = w;
  c.height = h;
  const ctx = c.getContext("2d", { willReadFrequently: true });
  ctx.drawImage(img, 0, 0, w, h);
  return { canvas: c, ctx, data: ctx.getImageData(0, 0, w, h) };
}

function colorDist(a, b, i) {
  const dr = a[i] - b[i];
  const dg = a[i + 1] - b[i + 1];
  const db = a[i + 2] - b[i + 2];
  return Math.sqrt(dr * dr + dg * dg + db * db);
}

/**
 * Analyze two images. sensitivity 0–100 (higher = fewer / stronger changes).
 * Returns regions sorted by area descending.
 */
export async function analyzeImageDiff(urlA, urlB, sensitivity = 35) {
  const [imgA, imgB] = await Promise.all([loadHTMLImage(urlA), loadHTMLImage(urlB)]);
  const w = Math.max(imgA.naturalWidth || imgA.width, imgB.naturalWidth || imgB.width, 1);
  const h = Math.max(imgA.naturalHeight || imgA.height, imgB.naturalHeight || imgB.height, 1);

  // Cap analysis resolution for speed.
  const maxSide = 640;
  const scale = Math.min(1, maxSide / Math.max(w, h));
  const aw = Math.max(1, Math.round(w * scale));
  const ah = Math.max(1, Math.round(h * scale));

  const a = canvasFromImage(imgA, aw, ah);
  const b = canvasFromImage(imgB, aw, ah);
  const threshold = 8 + (sensitivity / 100) * 70; // ~8–78

  const mask = new Uint8Array(aw * ah);
  let changed = 0;
  for (let y = 0; y < ah; y++) {
    for (let x = 0; x < aw; x++) {
      const i = (y * aw + x) * 4;
      if (colorDist(a.data.data, b.data.data, i) >= threshold) {
        mask[y * aw + x] = 1;
        changed++;
      }
    }
  }

  const regions = findRegions(mask, aw, ah, scale)
    .sort((r, s) => s.area - r.area)
    .slice(0, 12)
    .map((r, idx) => ({
      id: idx + 1,
      x: r.x,
      y: r.y,
      w: r.w,
      h: r.h,
      areaPct: (r.area / (aw * ah)) * 100,
      label: regionLabel(r, aw, ah),
    }));

  return {
    width: w,
    height: h,
    analysisWidth: aw,
    analysisHeight: ah,
    scale,
    percentChanged: (changed / (aw * ah)) * 100,
    changedPixels: changed,
    threshold,
    regions,
    imgA,
    imgB,
  };
}

function regionLabel(r, aw, ah) {
  const cx = (r.x + r.w / 2) / aw;
  const cy = (r.y + r.h / 2) / ah;
  const horiz = cx < 0.33 ? "left" : cx > 0.66 ? "right" : "center";
  const vert = cy < 0.33 ? "top" : cy > 0.66 ? "bottom" : "middle";
  if (vert === "middle" && horiz === "center") return "Center region";
  return `${vert[0].toUpperCase()}${vert.slice(1)}-${horiz} region`;
}

function findRegions(mask, w, h, scale) {
  const visited = new Uint8Array(mask.length);
  const regions = [];
  const minArea = Math.max(12, Math.floor((w * h) * 0.0008));

  for (let y = 0; y < h; y++) {
    for (let x = 0; x < w; x++) {
      const start = y * w + x;
      if (!mask[start] || visited[start]) continue;

      let minX = x,
        maxX = x,
        minY = y,
        maxY = y,
        area = 0;
      const queue = [start];
      visited[start] = 1;

      while (queue.length) {
        const p = queue.pop();
        const px = p % w;
        const py = (p - px) / w;
        area++;
        if (px < minX) minX = px;
        if (px > maxX) maxX = px;
        if (py < minY) minY = py;
        if (py > maxY) maxY = py;
        for (const [dx, dy] of [
          [1, 0],
          [-1, 0],
          [0, 1],
          [0, -1],
        ]) {
          const nx = px + dx;
          const ny = py + dy;
          if (nx < 0 || ny < 0 || nx >= w || ny >= h) continue;
          const ni = ny * w + nx;
          if (!mask[ni] || visited[ni]) continue;
          visited[ni] = 1;
          queue.push(ni);
        }
      }

      if (area < minArea) continue;
      // Pad slightly and map back to full-res coords.
      const pad = 2;
      const rx = Math.max(0, minX - pad) / scale;
      const ry = Math.max(0, minY - pad) / scale;
      const rw = Math.min(w - (minX - pad), maxX - minX + 1 + pad * 2) / scale;
      const rh = Math.min(h - (minY - pad), maxY - minY + 1 + pad * 2) / scale;
      regions.push({
        x: rx,
        y: ry,
        w: rw,
        h: rh,
        area: area / (scale * scale),
      });
    }
  }

  // Merge heavily overlapping boxes.
  return mergeOverlaps(regions);
}

function mergeOverlaps(regions) {
  const out = regions.slice();
  let merged = true;
  while (merged) {
    merged = false;
    for (let i = 0; i < out.length; i++) {
      for (let j = i + 1; j < out.length; j++) {
        if (overlapRatio(out[i], out[j]) > 0.25) {
          const a = out[i];
          const b = out[j];
          const x1 = Math.min(a.x, b.x);
          const y1 = Math.min(a.y, b.y);
          const x2 = Math.max(a.x + a.w, b.x + b.w);
          const y2 = Math.max(a.y + a.h, b.y + b.h);
          out[i] = { x: x1, y: y1, w: x2 - x1, h: y2 - y1, area: a.area + b.area };
          out.splice(j, 1);
          merged = true;
          break;
        }
      }
      if (merged) break;
    }
  }
  return out;
}

function overlapRatio(a, b) {
  const x1 = Math.max(a.x, b.x);
  const y1 = Math.max(a.y, b.y);
  const x2 = Math.min(a.x + a.w, b.x + b.w);
  const y2 = Math.min(a.y + a.h, b.y + b.h);
  if (x2 <= x1 || y2 <= y1) return 0;
  const inter = (x2 - x1) * (y2 - y1);
  const smaller = Math.min(a.w * a.h, b.w * b.h) || 1;
  return inter / smaller;
}

export function buildImageSummary(analysis, fromVer, toVer) {
  if (!analysis) return "Comparing image versions.";
  const pct = analysis.percentChanged.toFixed(1);
  const n = analysis.regions.length;
  if (analysis.percentChanged < 0.15) {
    return `<strong>Summary:</strong> v${fromVer} and v${toVer} look nearly identical (${pct}% of pixels differ).`;
  }
  if (!n) {
    return `<strong>Summary:</strong> about ${pct}% of pixels changed between v${fromVer} and v${toVer}, but no large clustered regions stood out.`;
  }
  const top = analysis.regions
    .slice(0, 3)
    .map((r) => r.label.toLowerCase())
    .join(", ");
  return `<strong>Summary:</strong> ${pct}% of pixels changed across ${n} region${n === 1 ? "" : "s"}. Largest changes: ${top}.`;
}

export function renderOverlays(regions, show, activeId = null) {
  if (!show || !regions?.length) return "";
  return regions
    .map((r) => {
      const active = activeId === r.id ? " active" : "";
      return `<div class="img-region${active}" data-region="${r.id}" style="left:${pct(r.x, r._bw)}%;top:${pct(r.y, r._bh)}%;width:${pct(r.w, r._bw)}%;height:${pct(r.h, r._bh)}%">
        <span class="img-region-num">${r.id}</span>
      </div>`;
    })
    .join("");
}

function pct(v, total) {
  return total ? ((v / total) * 100).toFixed(3) : 0;
}

export function withBounds(regions, width, height) {
  return (regions || []).map((r) => ({ ...r, _bw: width, _bh: height }));
}

/** Draw difference mode into a canvas element. */
export function paintDifference(canvas, analysis, sensitivity, showRegions) {
  if (!canvas || !analysis) return;
  const { imgA, imgB, width, height } = analysis;
  const maxW = 900;
  const scale = Math.min(1, maxW / width);
  const w = Math.max(1, Math.round(width * scale));
  const h = Math.max(1, Math.round(height * scale));
  canvas.width = w;
  canvas.height = h;
  const ctx = canvas.getContext("2d", { willReadFrequently: true });
  ctx.drawImage(imgB, 0, 0, w, h);
  const base = ctx.getImageData(0, 0, w, h);
  const a = canvasFromImage(imgA, w, h).data.data;
  const b = base.data;
  const threshold = 8 + (sensitivity / 100) * 70;

  for (let i = 0; i < b.length; i += 4) {
    const d = colorDist(a, b, i);
    if (d >= threshold) {
      // Highlight changed pixels in accent purple/magenta.
      b[i] = Math.min(255, 180 + d);
      b[i + 1] = Math.min(255, 90 + d * 0.3);
      b[i + 2] = Math.min(255, 220);
      b[i + 3] = 255;
    } else {
      // Dim unchanged.
      const g = 0.25 * b[i] + 0.45 * b[i + 1] + 0.15 * b[i + 2];
      b[i] = g * 0.35;
      b[i + 1] = g * 0.35;
      b[i + 2] = g * 0.4;
      b[i + 3] = 255;
    }
  }
  ctx.putImageData(base, 0, 0);

  if (showRegions && analysis.regions?.length) {
    ctx.lineWidth = 2;
    ctx.font = "bold 14px Inter, sans-serif";
    analysis.regions.forEach((r) => {
      const x = r.x * scale;
      const y = r.y * scale;
      const rw = r.w * scale;
      const rh = r.h * scale;
      ctx.strokeStyle = "rgba(180,140,255,0.95)";
      ctx.strokeRect(x, y, rw, rh);
      ctx.fillStyle = "rgba(180,140,255,0.95)";
      ctx.beginPath();
      ctx.arc(x + 12, y + 12, 11, 0, Math.PI * 2);
      ctx.fill();
      ctx.fillStyle = "#111";
      ctx.fillText(String(r.id), x + 8, y + 16);
    });
  }
}
