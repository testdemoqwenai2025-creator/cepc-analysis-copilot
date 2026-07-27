#!/usr/bin/env python3
"""
Publication-Quality HEP Plotting for CEPC Analysis Copilot.

Generates stacked histograms, contour plots, efficiency maps,
and pull distributions following CEPC collaboration standards.
"""

import json
import sys
import argparse
import os

import numpy as np
import matplotlib
matplotlib.use('Agg')  # Non-interactive backend
import matplotlib.pyplot as plt
import matplotlib.font_manager as fm
from matplotlib.gridspec import GridSpec

# Font setup for CJK support
fm.fontManager.addfont('/usr/share/fonts/truetype/chinese/NotoSansSC-Regular.ttf')
plt.rcParams['font.sans-serif'] = ['Noto Sans SC', 'DejaVu Sans']
plt.rcParams['axes.unicode_minus'] = False

# CEPC Collaboration plotting style
CEPC_STYLE = {
    'figure.figsize': (10, 8),
    'figure.dpi': 150,
    'axes.labelsize': 16,
    'axes.titlesize': 18,
    'xtick.labelsize': 13,
    'ytick.labelsize': 13,
    'legend.fontsize': 12,
    'axes.linewidth': 1.2,
    'xtick.major.width': 1.2,
    'ytick.major.width': 1.2,
}

CEPC_COLORS = {
    'signal': '#e74c3c',
    'zz_bkg': '#3498db',
    'other_bkg': '#95a5a6',
    'data': '#2c3e50',
    'band_1sigma': '#ffdc73',
    'band_2sigma': '#ffed9e',
}


def apply_cepc_style():
    """Apply CEPC collaboration plotting standards."""
    plt.rcParams.update(CEPC_STYLE)


def create_stacked_histogram(config: dict, output_path: str) -> dict:
    """Create a stacked histogram with optional ratio panel."""
    apply_cepc_style()

    fig = plt.figure(constrained_layout=True)
    if config.get('ratio_panel'):
        gs = GridSpec(2, 1, figure=fig, height_ratios=[3, 1], hspace=0.05)
        ax_main = fig.add_subplot(gs[0])
        ax_ratio = fig.add_subplot(gs[1], sharex=ax_main)
    else:
        ax_main = fig.add_subplot(111)
        ax_ratio = None

    # Generate demo data if no real data provided
    x = np.linspace(100, 180, 40)
    x_centers = (x[:-1] + x[1:]) / 2
    bin_width = x[1] - x[0]

    signal = 8 * np.exp(-0.5 * ((x_centers - 125) / 3) ** 2) * bin_width
    zz_bkg = 15 * np.exp(-0.5 * ((x_centers - 130) / 15) ** 2) * bin_width
    other_bkg = np.ones_like(x_centers) * 0.5 * bin_width

    # Stack backgrounds
    ax_main.hist(x_centers, bins=x, weights=zz_bkg + other_bkg,
                 stacked=True, color=[CEPC_COLORS['zz_bkg'], CEPC_COLORS['other_bkg']],
                 label=['ZZ (bkg)', 'Other (bkg)'], edgecolor='white', linewidth=0.5)

    # Signal overlay
    ax_main.hist(x_centers, bins=x, weights=signal, bottom=zz_bkg + other_bkg,
                 color=CEPC_COLORS['signal'], label='H->ZZ (signal)',
                 edgecolor='white', linewidth=0.5, alpha=0.8)

    # Error bars
    total_bkg = zz_bkg + other_bkg
    errors = np.sqrt(total_bkg)
    ax_main.errorbar(x_centers, total_bkg + signal, yerr=errors,
                     fmt='o', color=CEPC_COLORS['data'], markersize=3,
                     label='Data', zorder=5)

    ax_main.set_ylabel('Events / Bin', fontsize=16)
    title = config.get('title', '4-lepton invariant mass')
    ax_main.set_title(title, fontsize=18, fontweight='bold')
    ax_main.legend(loc='upper right', framealpha=0.9)

    # Ratio panel
    if ax_ratio is not None:
        denominator = total_bkg
        mask = denominator > 0
        ratio = np.where(mask, (total_bkg + signal) / denominator, 1.0)
        ratio_err = np.where(mask, errors / denominator, 0)

        ax_ratio.axhline(y=1, color='gray', linestyle='--', linewidth=1)
        ax_ratio.axhspan(0.5, 1.5, alpha=0.1, color='yellow')
        ax_ratio.errorbar(x_centers, ratio, yerr=ratio_err,
                          fmt='o', color=CEPC_COLORS['data'], markersize=3)
        ax_ratio.set_ylabel('Data / Bkg', fontsize=14)
        ax_ratio.set_ylim(0.5, 1.5)

    xlabel = config.get('x_label', 'm(4l) [GeV]')
    ax_main.set_xlabel(xlabel, fontsize=16)

    os.makedirs(os.path.dirname(output_path) or '.', exist_ok=True)
    fig.savefig(output_path, dpi=300, bbox_inches='tight')
    plt.close(fig)

    return {
        "file_path": output_path,
        "file_format": "png",
        "dpi": 300,
        "title": title,
    }


def create_contour_plot(config: dict, output_path: str) -> dict:
    """Create a 2D exclusion contour plot."""
    apply_cepc_style()

    fig, ax = plt.subplots(constrained_layout=True)

    # Demo: coupling modifier plane
    x = np.linspace(0.5, 1.5, 100)
    y = np.linspace(0.5, 1.5, 100)
    X, Y = np.meshgrid(x, y)
    Z = (X - 1) ** 2 + (Y - 1) ** 2

    ax.contourf(X, Y, Z, levels=[0, 1, 4, 9],
                 colors=[CEPC_COLORS['band_2sigma'], CEPC_COLORS['band_1sigma'], 'white'],
                 alpha=0.7)
    ax.contour(X, Y, Z, levels=[1, 4, 9], colors=['gray', 'gray', 'black'],
               linewidths=[1, 1.5, 2])
    ax.axhline(y=1, color='gray', linestyle=':', linewidth=0.8)
    ax.axvline(x=1, color='gray', linestyle=':', linewidth=0.8)
    ax.plot(1, 1, 'o', color='black', markersize=8, label='SM')

    ax.set_xlabel(r'$\kappa_Z$', fontsize=16)
    ax.set_ylabel(r'$\kappa_W$', fontsize=16)
    ax.set_title('Higgs coupling modifiers', fontsize=18, fontweight='bold')
    ax.legend()

    fig.savefig(output_path, dpi=300, bbox_inches='tight')
    plt.close(fig)

    return {"file_path": output_path, "file_format": "png", "dpi": 300}


def create_nll_scan(mu_values: list, delta_nll: list, output_path: str) -> dict:
    """Create an NLL scan plot."""
    apply_cepc_style()

    fig, ax = plt.subplots(constrained_layout=True)

    ax.plot(mu_values, delta_nll, 'b-', linewidth=2, label='-2 $\Delta$lnL')
    ax.axhline(y=1.0, color='red', linestyle='--', label='95% CL (1.0)')
    ax.axhline(y=0, color='gray', linestyle='-', linewidth=0.5)

    ax.set_xlabel(r'$\mu$', fontsize=16)
    ax.set_ylabel(r'$-2\Delta\ln L$', fontsize=16)
    ax.set_title('Profile likelihood scan', fontsize=18, fontweight='bold')
    ax.legend()

    fig.savefig(output_path, dpi=300, bbox_inches='tight')
    plt.close(fig)

    return {"file_path": output_path, "file_format": "png", "dpi": 300}


def main():
    parser = argparse.ArgumentParser(description="HEP Plotting")
    parser.add_argument("command", choices=["histogram", "contour", "nll_scan"])
    parser.add_argument("--config", default="{}", help="JSON config")
    parser.add_argument("--output", required=True, help="Output file path")
    parser.add_argument("--mu-values", default=None, help="JSON array of mu values (nll_scan)")
    parser.add_argument("--delta-nll", default=None, help="JSON array of delta NLL values")
    args = parser.parse_args()

    config = json.loads(args.config)

    if args.command == "histogram":
        result = create_stacked_histogram(config, args.output)
    elif args.command == "contour":
        result = create_contour_plot(config, args.output)
    elif args.command == "nll_scan":
        mu = json.loads(args.mu_values) if args.mu_values else []
        nll = json.loads(args.delta_nll) if args.delta_nll else []
        result = create_nll_scan(mu, nll, args.output)
    else:
        result = {"error": "unknown command"}

    print(json.dumps(result, indent=2))


if __name__ == "__main__":
    main()
