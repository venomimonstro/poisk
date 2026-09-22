"use client";

import { useEffect, useRef, useState } from "react";
import * as maplibregl from "maplibre-gl";
import type { StyleSpecification } from "maplibre-gl";
import { PMTiles, Protocol } from "pmtiles";
import ReviewPanel from "./review-panel";
import MapSearchPanel, { type MapSearchResult } from "./search-panel";

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

type GeoResult = MapSearchResult;

type GeoCluster = {
  id: string;
  latitude: number;
  longitude: number;
  count: number;
  place_ids?: number[];
};

type ViewportResponse = {
  total: number;
  took_ms: number;
  results: GeoResult[];
  clusters: GeoCluster[];
};

export default function MapPage() {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const mapRef = useRef<maplibregl.Map | null>(null);
  const markerRef = useRef<maplibregl.Marker[]>([]);
  const requestRef = useRef<AbortController | null>(null);
  const resultsRef = useRef<Map<number, GeoResult>>(new Map());
  const [status, setStatus] = useState("Загружаем карту…");
  const [version, setVersion] = useState("");
  const [selected, setSelected] = useState<GeoResult | null>(null);
  const [geoMeta, setGeoMeta] = useState("");

  function selectOrganization(item: GeoResult) {
    setSelected(item);
    if (item.longitude !== undefined && item.latitude !== undefined && mapRef.current) {
      mapRef.current.easeTo({ center: [item.longitude, item.latitude], zoom: Math.max(mapRef.current.getZoom(), 15) });
    }
  }

  useEffect(() => {
    let cancelled = false;
    const protocol = new Protocol();
    maplibregl.addProtocol("pmtiles", protocol.tile);

    async function refreshOrganizations(map: maplibregl.Map) {
      const bounds = map.getBounds();
      if (!bounds) return;
      requestRef.current?.abort();
      const controller = new AbortController();
      requestRef.current = controller;
      const params = new URLSearchParams({
        min_lat: bounds.getSouth().toFixed(6),
        min_lon: bounds.getWest().toFixed(6),
        max_lat: bounds.getNorth().toFixed(6),
        max_lon: bounds.getEast().toFixed(6),
        zoom: map.getZoom().toFixed(2),
        limit: "200",
      });
      try {
        const response = await fetch(`/api/geo/viewport?${params.toString()}`, {
          signal: controller.signal,
          headers: { Accept: "application/json" },
          cache: "no-store",
        });
        if (!response.ok) throw new Error("geo_viewport_unavailable");
        const data = (await response.json()) as ViewportResponse;
        if (cancelled || controller.signal.aborted) return;
        resultsRef.current = new Map(data.results.map((item) => [item.id, item]));
        markerRef.current.forEach((marker) => marker.remove());
        markerRef.current = data.clusters.map((cluster) => {
          const button = document.createElement("button");
          button.type = "button";
          button.className = cluster.count > 1 ? "geoClusterMarker" : "geoPlaceMarker";
          button.textContent = cluster.count > 1 ? String(cluster.count) : "";
          button.setAttribute("aria-label", cluster.count > 1 ? `${cluster.count} организаций` : "Организация");
          button.addEventListener("click", () => {
            if (cluster.count > 1) {
              map.easeTo({ center: [cluster.longitude, cluster.latitude], zoom: Math.min(map.getZoom() + 2, map.getMaxZoom()) });
              return;
            }
            const placeID = cluster.place_ids?.[0];
            if (placeID) {
              const item = resultsRef.current.get(placeID);
              if (item) selectOrganization(item);
            }
          });
          return new maplibregl.Marker({ element: button, anchor: "center" })
            .setLngLat([cluster.longitude, cluster.latitude])
            .addTo(map);
        });
        setGeoMeta(data.total > data.results.length ? `Показаны ${data.results.length} из ${data.total}` : `${data.results.length} организаций`);
      } catch (error) {
        if (error instanceof DOMException && error.name === "AbortError") return;
        console.error("geo viewport failed", error);
        if (!cancelled) setGeoMeta("Организации временно недоступны");
      }
    }

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
            void refreshOrganizations(map);
          }
        });
        map.on("moveend", () => void refreshOrganizations(map));
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
      requestRef.current?.abort();
      markerRef.current.forEach((marker) => marker.remove());
      markerRef.current = [];
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
          {geoMeta && <span className="mapVersion"> · {geoMeta}</span>}
        </div>
      </header>
      <div className="mapViewport" ref={containerRef} aria-label="Интерактивная карта" />
      <MapSearchPanel onSelect={selectOrganization} />
      {selected && (
        <aside className="geoCard" aria-label="Организация" style={{ maxHeight: "calc(100vh - 100px)", overflowY: "auto" }}>
          <button className="geoCardClose" type="button" onClick={() => setSelected(null)} aria-label="Закрыть">×</button>
          <div className="geoCardCategory">{selected.category_key || "Организация"}</div>
          <strong className="geoCardTitle">{selected.name}</strong>
          {selected.address && <div className="geoCardAddress">{selected.address}</div>}
          <div className="geoCardMeta">Источников: {selected.source_count}</div>
          <div className="geoCardActions">
            {selected.website && <a href={selected.website} rel="noopener noreferrer">Сайт</a>}
            {selected.phone && <a href={`tel:${selected.phone}`}>Позвонить</a>}
          </div>
          <ReviewPanel placeID={selected.id} />
        </aside>
      )}
      {status && <div className="mapStatus" role="status">{status}</div>}
    </main>
  );
}
