"use client";

import { useEffect, useRef, useState } from "react";
import * as maplibregl from "maplibre-gl";
import type { StyleSpecification } from "maplibre-gl";
import { PMTiles, Protocol } from "pmtiles";

type MapConfig = {
  version: string;
  pmtiles_url: string;
  style_url: string;
  pmtiles_sha256: string;
  pmtiles_size: number;
  bounds: [number, number, number, number];
  min_zoom: number;
  max_zoom: number;
  center: [number, number, number];
  source_name: string;
  attribution_html: string;
};

export default function MapPage() {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const mapRef = useRef<maplibregl.Map | null>(null);
  const [status, setStatus] = useState("Загружаем карту…");
  const [version, setVersion] = useState("");

  useEffect(() => {
    let cancelled = false;
    const protocol = new Protocol();
    maplibregl.addProtocol("pmtiles", protocol.tile);

    async function boot() {
      try {
        const configResponse = await fetch("/api/map/config", {
          headers: { Accept: "application/json" },
          cache: "no-store",
        });
        if (!configResponse.ok) throw new Error("map_config_unavailable");
        const config = (await configResponse.json()) as MapConfig;
        if (cancelled || !containerRef.current) return;

        const styleResponse = await fetch(config.style_url, {
          headers: { Accept: "application/json" },
          cache: "force-cache",
        });
        if (!styleResponse.ok) throw new Error("map_style_unavailable");
        const style = (await styleResponse.json()) as StyleSpecification;
        const source = style.sources?.[config.source_name];
        if (!source || source.type !== "vector") throw new Error("map_source_invalid");

        const archiveURL = new URL(config.pmtiles_url, window.location.origin).toString();
        const archive = new PMTiles(archiveURL);
        protocol.add(archive);

        source.url = `pmtiles://${archiveURL}`;
        delete source.tiles;
        source.attribution = config.attribution_html;

        const map = new maplibregl.Map({
          container: containerRef.current,
          style,
          center: [config.center[0], config.center[1]],
          zoom: config.center[2],
          minZoom: config.min_zoom,
          maxZoom: config.max_zoom,
          maxBounds: [
            [config.bounds[0], config.bounds[1]],
            [config.bounds[2], config.bounds[3]],
          ],
          attributionControl: true,
          hash: "map",
        });
        mapRef.current = map;
        map.addControl(new maplibregl.NavigationControl({ visualizePitch: true }), "top-right");
        map.addControl(new maplibregl.ScaleControl({ unit: "metric" }), "bottom-left");
        map.once("load", () => {
          if (!cancelled) {
            setVersion(config.version);
            setStatus("");
          }
        });
        map.on("error", (event) => {
          console.error("map error", event.error);
          if (!cancelled) setStatus("Не удалось загрузить часть карты.");
        });
      } catch (error) {
        console.error("map bootstrap failed", error);
        if (!cancelled) setStatus("Карта пока недоступна.");
      }
    }

    void boot();
    return () => {
      cancelled = true;
      mapRef.current?.remove();
      mapRef.current = null;
      maplibregl.removeProtocol("pmtiles");
    };
  }, []);

  return (
    <main className="mapPage">
      <header className="mapHeader">
        <a className="mapBrand" href="/">ПОИСК</a>
        <div>
          <strong>Карта</strong>
          {version && <span className="mapVersion"> версия {version}</span>}
        </div>
      </header>
      <div className="mapViewport" ref={containerRef} aria-label="Интерактивная карта" />
      {status && <div className="mapStatus" role="status">{status}</div>}
    </main>
  );
}
