import { VendorsSection } from '../features/admin/vendors/VendorsSection';

export function AdminPumpVendorsPage() {
  return (
    <div className="space-y-6">
      <VendorsSection initialCollection="solarPump" collections={['solarPump']} />
    </div>
  );
}
