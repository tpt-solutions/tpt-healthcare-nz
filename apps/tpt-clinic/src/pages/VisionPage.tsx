import AppShell from '@/components/AppShell';

export default function VisionPage() {
  return (
    <AppShell title="Optometry / Ophthalmology">
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <div className="rounded-xl border border-primary-200 bg-white p-5 shadow-sm">
          <p className="text-sm font-medium text-secondary-500">ACC Claims</p>
          <p className="mt-2 text-3xl font-bold text-secondary-900">0</p>
        </div>
        <div className="rounded-xl border border-green-200 bg-white p-5 shadow-sm">
          <p className="text-sm font-medium text-secondary-500">Current Prescriptions</p>
          <p className="mt-2 text-3xl font-bold text-secondary-900">0</p>
        </div>
        <div className="rounded-xl border border-amber-200 bg-white p-5 shadow-sm">
          <p className="text-sm font-medium text-secondary-500">Dispensing Orders</p>
          <p className="mt-2 text-3xl font-bold text-secondary-900">0</p>
        </div>
        <div className="rounded-xl border border-secondary-200 bg-white p-5 shadow-sm">
          <p className="text-sm font-medium text-secondary-500">Exams Today</p>
          <p className="mt-2 text-3xl font-bold text-secondary-900">0</p>
        </div>
      </div>

      <div className="mt-6 grid grid-cols-1 gap-6 lg:grid-cols-2">
        {/* Refraction / Prescriptions */}
        <div className="rounded-xl bg-white p-6 shadow-sm ring-1 ring-secondary-200">
          <h2 className="text-lg font-semibold text-secondary-900">Refraction & Prescriptions</h2>
          <p className="mt-2 text-sm text-secondary-500">
            Manage spectacle and contact lens prescriptions, Snellen visual acuity conversion,
            and spherical equivalent calculations for NZ optometry practice.
          </p>
          <div className="mt-4 space-y-2">
            <p className="text-sm text-secondary-500">
              <span className="font-medium text-secondary-700">Key features:</span>
            </p>
            <ul className="list-inside list-disc text-sm text-secondary-500">
              <li>Sphere, cylinder, axis, prism, and ADD power entries (0.25D steps)</li>
              <li>Distance, near, and intermediate prescriptions</li>
              <li>Snellen & LogMAR conversion utilities</li>
              <li>Spectacle and contact lens prescription types</li>
              <li>Standard 1-year expiry with renewal tracking</li>
            </ul>
          </div>
        </div>

        {/* Ophthalmic Examinations */}
        <div className="rounded-xl bg-white p-6 shadow-sm ring-1 ring-secondary-200">
          <h2 className="text-lg font-semibold text-secondary-900">Ophthalmic Examinations</h2>
          <p className="mt-2 text-sm text-secondary-500">
            Comprehensive eye exam records covering anterior/posterior segment findings,
            tonometry, OCT imaging, and visual fields for NZ ophthalmology.
          </p>
          <div className="mt-4 space-y-2">
            <p className="text-sm text-secondary-500">
              <span className="font-medium text-secondary-700">Key features:</span>
            </p>
            <ul className="list-inside list-disc text-sm text-secondary-500">
              <li>Visual acuity (distance, near, pinhole) per eye</li>
              <li>Goldmann applanation, non-contact & rebound tonometry</li>
              <li>Lens status: phakic, nuclear sclerotic, PSC, PCIOL, etc.</li>
              <li>Cup-to-disc ratio, optic disc appearance grading</li>
              <li>Macular status and OCT summary reporting</li>
            </ul>
          </div>
        </div>

        {/* Optical Dispensing */}
        <div className="rounded-xl bg-white p-6 shadow-sm ring-1 ring-secondary-200">
          <h2 className="text-lg font-semibold text-secondary-900">Optical Dispensing</h2>
          <p className="mt-2 text-sm text-secondary-500">
            Complete spectacle and contact lens dispensing workflow from order to collection,
            with frame selection, lens specifications, and pricing.
          </p>
          <div className="mt-4 space-y-2">
            <p className="text-sm text-secondary-500">
              <span className="font-medium text-secondary-700">Key features:</span>
            </p>
            <ul className="list-inside list-disc text-sm text-secondary-500">
              <li>Frame selection: full rim, semi-rimless, rimless, safety</li>
              <li>Lens types: single vision, bifocal, progressive, photochromic</li>
              <li>Lens index options from CR-39 to 1.74 ultra-high-index</li>
              <li>Contact lens orders: daily, monthly, RGP, ortho-K, scleral</li>
              <li>Order status lifecycle & warranty tracking</li>
            </ul>
          </div>
        </div>

        {/* ACC Claims */}
        <div className="rounded-xl bg-white p-6 shadow-sm ring-1 ring-secondary-200">
          <h2 className="text-lg font-semibold text-secondary-900">ACC Vision Claims</h2>
          <p className="mt-2 text-sm text-secondary-500">
            ACC claim management for eye injuries, spectacle replacement, and
            funded eye examinations under the NZ ACC framework.
          </p>
          <div className="mt-4 space-y-2">
            <p className="text-sm text-secondary-500">
              <span className="font-medium text-secondary-700">Key features:</span>
            </p>
            <ul className="list-inside list-disc text-sm text-secondary-500">
              <li>Claim types: eye exam, spectacle/CL replacement, surgical correction</li>
              <li>ACC procedure codes for vision (OPT101–OPT502)</li>
              <li>Claim lifecycle: draft → submitted → accepted/declined</li>
              <li>Multi-line item claims with GST breakdown</li>
              <li>Linked to prescriptions, exams, and dispensing orders</li>
            </ul>
          </div>
        </div>
      </div>
    </AppShell>
  );
}